// Package execsrc adapts any command-line tool that prints JSON into a
// source. An adapter is described entirely by a manifest file, so wiring a
// new site CLI in means dropping a JSON file into sources.d, not writing Go.
package execsrc

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tamnd/shirabe/pkg/schema"
)

// Op describes one invocation of the binary: how to build argv, how to read
// the output, and how to map records into cards.
type Op struct {
	// Args is the argv template after the binary name. Placeholders:
	// {query}, {url}, {id}, {n}.
	Args []string `json:"args"`
	// Output is the wire shape on stdout: "json" (one object), "array"
	// (top-level JSON array), or "jsonl" (default).
	Output string `json:"output,omitempty"`
	// Items optionally names a dot path to the record array inside a
	// "json" object, e.g. "data.items".
	Items string `json:"items,omitempty"`
	// Kind is the schema kind every record becomes.
	Kind schema.Kind `json:"kind"`
	// Map goes from card field to a dot path in the record. Card fields:
	// title, url, snippet, thumbnail, score, and body.<name> for the
	// kind-specific payload.
	Map map[string]string `json:"map"`
	// IDPattern is a regex with one capture group applied to the input URL
	// to fill {id}. Resolve ops only.
	IDPattern string `json:"id_pattern,omitempty"`
	// TimeoutMS overrides the engine's per-source deadline, within reason.
	TimeoutMS int `json:"timeout_ms,omitempty"`

	idRe *regexp.Regexp
}

// Manifest is one sources.d file.
type Manifest struct {
	Name     string   `json:"name"`
	Binary   string   `json:"binary"`
	Priority int      `json:"priority,omitempty"`
	Hosts    []string `json:"hosts,omitempty"`
	Intents  []string `json:"intents,omitempty"`
	Search   *Op      `json:"search,omitempty"`
	Resolve  *Op      `json:"resolve,omitempty"`
}

func (m *Manifest) validate() error {
	if m.Name == "" {
		return fmt.Errorf("manifest: missing name")
	}
	if m.Binary == "" {
		return fmt.Errorf("manifest %s: missing binary", m.Name)
	}
	if m.Search == nil && m.Resolve == nil {
		return fmt.Errorf("manifest %s: needs a search or resolve op", m.Name)
	}
	for label, op := range map[string]*Op{"search": m.Search, "resolve": m.Resolve} {
		if op == nil {
			continue
		}
		if len(op.Args) == 0 {
			return fmt.Errorf("manifest %s: %s: empty args", m.Name, label)
		}
		if op.Kind == "" || !schema.Known(op.Kind) {
			return fmt.Errorf("manifest %s: %s: unknown kind %q", m.Name, label, op.Kind)
		}
		switch op.Output {
		case "", "json", "jsonl", "array":
		default:
			return fmt.Errorf("manifest %s: %s: unknown output %q", m.Name, label, op.Output)
		}
		if op.Map["title"] == "" && op.Map["url"] == "" {
			return fmt.Errorf("manifest %s: %s: map needs at least title or url", m.Name, label)
		}
		if op.IDPattern != "" {
			re, err := regexp.Compile(op.IDPattern)
			if err != nil {
				return fmt.Errorf("manifest %s: %s: id_pattern: %w", m.Name, label, err)
			}
			if re.NumSubexp() < 1 {
				return fmt.Errorf("manifest %s: %s: id_pattern needs a capture group", m.Name, label)
			}
			op.idRe = re
		}
	}
	if m.Resolve != nil && len(m.Hosts) == 0 {
		return fmt.Errorf("manifest %s: resolve op needs hosts", m.Name)
	}
	return nil
}

// Parse reads and validates one manifest.
func Parse(data []byte, origin string) (*Manifest, error) {
	var m Manifest
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", origin, err)
	}
	if err := m.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", origin, err)
	}
	return &m, nil
}

// LoadFS parses every *.json under dir in fsys. Bad files are reported in
// errs and skipped; good ones still load.
func LoadFS(fsys fs.FS, dir string) (manifests []*Manifest, errs []error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, []error{err}
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := fs.ReadFile(fsys, filepath.Join(dir, e.Name()))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		m, err := Parse(data, e.Name())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		manifests = append(manifests, m)
	}
	return manifests, errs
}

// LoadDir parses every *.json in an on-disk directory. A missing directory
// is not an error.
func LoadDir(dir string) ([]*Manifest, []error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, nil
	}
	return LoadFS(os.DirFS(dir), ".")
}

// UserDir is where user manifests live.
func UserDir() string {
	if base, err := os.UserConfigDir(); err == nil {
		return filepath.Join(base, "shirabe", "sources.d")
	}
	return ""
}
