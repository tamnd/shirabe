package execsrc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
)

const (
	maxStdout   = 8 << 20 // cap a chatty CLI at 8 MB
	maxStderr   = 4 << 10
	availTTL    = 30 * time.Second
	defaultKind = schema.KindWeb
)

// Adapter runs one external CLI per the manifest. It implements
// source.Source, source.Availability, and source.Priority.
type Adapter struct {
	m *Manifest

	mu        sync.Mutex
	checkedAt time.Time
	present   bool
}

func New(m *Manifest) *Adapter { return &Adapter{m: m} }

func (a *Adapter) Name() string { return a.m.Name }

func (a *Adapter) Priority() int {
	if a.m.Priority != 0 {
		return a.m.Priority
	}
	return 100
}

func (a *Adapter) Caps() source.Caps {
	return source.Caps{
		Search:  a.m.Search != nil,
		Resolve: a.m.Resolve != nil,
		Hosts:   a.m.Hosts,
		Intents: a.m.Intents,
	}
}

// Available reports whether the binary is on PATH, cached briefly so a fanned
// out query does not stat the filesystem per source per request.
func (a *Adapter) Available() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if time.Since(a.checkedAt) < availTTL {
		return a.present
	}
	_, err := exec.LookPath(a.m.Binary)
	a.present, a.checkedAt = err == nil, time.Now()
	return a.present
}

func (a *Adapter) Search(ctx context.Context, q source.Query) ([]schema.Card, error) {
	if a.m.Search == nil {
		return nil, source.ErrNotHandled
	}
	vars := map[string]string{"query": q.Raw, "n": strconv.Itoa(max(q.Limit, 1))}
	cards, err := a.run(ctx, a.m.Search, vars)
	if err != nil {
		return nil, err
	}
	if q.Limit > 0 && len(cards) > q.Limit {
		cards = cards[:q.Limit]
	}
	return cards, nil
}

func (a *Adapter) Resolve(ctx context.Context, u *url.URL) ([]schema.Card, error) {
	op := a.m.Resolve
	if op == nil {
		return nil, source.ErrNotHandled
	}
	vars := map[string]string{"url": u.String(), "n": "1"}
	if op.idRe != nil {
		m := op.idRe.FindStringSubmatch(u.String())
		if m == nil {
			return nil, source.ErrNotHandled
		}
		vars["id"] = m[1]
	}
	return a.run(ctx, op, vars)
}

func (a *Adapter) run(ctx context.Context, op *Op, vars map[string]string) ([]schema.Card, error) {
	if op.TimeoutMS > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(op.TimeoutMS)*time.Millisecond)
		defer cancel()
	}
	argv := make([]string, len(op.Args))
	for i, arg := range op.Args {
		argv[i] = expand(arg, vars)
	}
	// Args go straight to the process as a vector; there is no shell and
	// nothing to inject into.
	cmd := exec.CommandContext(ctx, a.m.Binary, argv...)
	var stderr bytes.Buffer
	cmd.Stderr = &limitWriter{w: &stderr, n: maxStderr}
	cmd.WaitDelay = 2 * time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s: %w", a.m.Binary, err)
	}
	records, readErr := readRecords(io.LimitReader(stdout, maxStdout), op)
	waitErr := cmd.Wait()
	if waitErr != nil && len(records) == 0 {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = waitErr.Error()
		}
		return nil, fmt.Errorf("%s: %s", a.m.Binary, firstLine(msg))
	}
	if readErr != nil && len(records) == 0 {
		return nil, fmt.Errorf("%s: bad output: %v", a.m.Binary, readErr)
	}

	cards := make([]schema.Card, 0, len(records))
	for _, rec := range records {
		if c, ok := a.card(op, rec); ok {
			cards = append(cards, c)
		}
	}
	return cards, nil
}

// card maps one decoded record into a Card via the manifest's field map.
func (a *Adapter) card(op *Op, rec any) (schema.Card, bool) {
	c := schema.Card{Kind: op.Kind, Source: a.m.Name, FetchedAt: time.Now()}
	if c.Kind == "" {
		c.Kind = defaultKind
	}
	body := map[string]any{}
	for field, path := range op.Map {
		v := get(rec, path)
		if v == nil {
			continue
		}
		switch {
		case field == "title":
			c.Title = asString(v)
		case field == "url":
			c.URL = asString(v)
		case field == "snippet":
			c.Snippet = asString(v)
		case field == "thumbnail":
			c.Thumbnail = asString(v)
		case field == "score":
			if f, ok := asFloat(v); ok {
				c.Score = f
			}
		case strings.HasPrefix(field, "body."):
			body[strings.TrimPrefix(field, "body.")] = v
		}
	}
	if len(body) > 0 {
		c.Body = body
	}
	if c.Title == "" && c.URL == "" {
		return c, false
	}
	return c, true
}

func readRecords(r io.Reader, op *Op) ([]any, error) {
	switch op.Output {
	case "json":
		var doc any
		if err := json.NewDecoder(r).Decode(&doc); err != nil {
			return nil, err
		}
		if op.Items != "" {
			doc = get(doc, op.Items)
		}
		if arr, ok := doc.([]any); ok {
			return arr, nil
		}
		if doc == nil {
			return nil, nil
		}
		return []any{doc}, nil
	case "array":
		var arr []any
		if err := json.NewDecoder(r).Decode(&arr); err != nil {
			return nil, err
		}
		return arr, nil
	default: // jsonl
		var out []any
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64<<10), 4<<20)
		for sc.Scan() {
			line := bytes.TrimSpace(sc.Bytes())
			if len(line) == 0 {
				continue
			}
			var rec any
			if err := json.Unmarshal(line, &rec); err != nil {
				return out, err
			}
			out = append(out, rec)
		}
		return out, sc.Err()
	}
}

func expand(arg string, vars map[string]string) string {
	for k, v := range vars {
		arg = strings.ReplaceAll(arg, "{"+k+"}", v)
	}
	return arg
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

type limitWriter struct {
	w io.Writer
	n int
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if l.n <= 0 {
		return len(p), nil
	}
	if len(p) > l.n {
		p = p[:l.n]
	}
	n, err := l.w.Write(p)
	l.n -= n
	return len(p), err
}
