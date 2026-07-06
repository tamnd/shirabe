package execsrc

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
)

// fakeBin drops an executable shell script on PATH that prints stdout and
// exits with code.
func fakeBin(t *testing.T, name, stdout string, code int) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\ncat <<'EOF'\n" + stdout + "\nEOF\nexit " + map[int]string{0: "0", 1: "1"}[code] + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func manifest(t *testing.T, raw string) *Manifest {
	t.Helper()
	m, err := Parse([]byte(raw), "test.json")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

const ytManifest = `{
	"name": "faketube",
	"binary": "faketube-cli",
	"hosts": ["faketube.example"],
	"search": {
		"args": ["search", "{query}", "-n", "{n}"],
		"output": "jsonl",
		"kind": "video",
		"map": {
			"title": "title",
			"url": "url",
			"thumbnail": "thumb",
			"body.channel": "channel.name",
			"body.views": "views"
		}
	},
	"resolve": {
		"args": ["video", "{id}"],
		"output": "json",
		"kind": "video",
		"id_pattern": "[?&]v=([A-Za-z0-9_-]+)",
		"map": {"title": "title", "url": "url"}
	}
}`

func TestSearchMapsRecords(t *testing.T) {
	fakeBin(t, "faketube-cli",
		`{"title":"one","url":"https://a/1","thumb":"https://t/1","channel":{"name":"ch"},"views":42}
{"title":"two","url":"https://a/2","views":"1,234"}`, 0)
	a := New(manifest(t, ytManifest))
	if !a.Available() {
		t.Fatal("binary should be available")
	}
	cards, err := a.Search(context.Background(), source.Query{Raw: "x", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("want 2 cards, got %d", len(cards))
	}
	if cards[0].Kind != schema.KindVideo || cards[0].Title != "one" || cards[0].Thumbnail != "https://t/1" {
		t.Fatalf("bad card: %+v", cards[0])
	}
	body := cards[0].Body.(map[string]any)
	if body["channel"] != "ch" || body["views"] != float64(42) {
		t.Fatalf("bad body: %#v", body)
	}
}

func TestSearchLimitApplied(t *testing.T) {
	fakeBin(t, "faketube-cli",
		`{"title":"1","url":"https://a/1"}
{"title":"2","url":"https://a/2"}
{"title":"3","url":"https://a/3"}`, 0)
	a := New(manifest(t, ytManifest))
	cards, err := a.Search(context.Background(), source.Query{Raw: "x", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("want limit 2, got %d", len(cards))
	}
}

func TestResolveExtractsID(t *testing.T) {
	fakeBin(t, "faketube-cli", `{"title":"vid","url":"https://faketube.example/watch?v=abc"}`, 0)
	a := New(manifest(t, ytManifest))
	u, _ := url.Parse("https://faketube.example/watch?v=abc")
	cards, err := a.Resolve(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || cards[0].Title != "vid" {
		t.Fatalf("bad resolve: %+v", cards)
	}
	// No id in the URL: pattern misses, adapter passes.
	u2, _ := url.Parse("https://faketube.example/about")
	if _, err := a.Resolve(context.Background(), u2); err != source.ErrNotHandled {
		t.Fatalf("want ErrNotHandled, got %v", err)
	}
}

func TestExitFailureSurfacesStderr(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho 'rate limited' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "faketube-cli"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	a := New(manifest(t, ytManifest))
	_, err := a.Search(context.Background(), source.Query{Raw: "x", Limit: 2})
	if err == nil || err.Error() != "faketube-cli: rate limited" {
		t.Fatalf("want stderr in error, got %v", err)
	}
}

func TestMissingBinaryUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	a := New(manifest(t, ytManifest))
	if a.Available() {
		t.Fatal("binary should be missing")
	}
}

func TestLoadFSSkipsBadManifests(t *testing.T) {
	fsys := fstest.MapFS{
		"sources.d/good.json": {Data: []byte(ytManifest)},
		"sources.d/bad.json":  {Data: []byte(`{"name":"x"}`)},
	}
	manifests, errs := LoadFS(fsys, "sources.d")
	if len(manifests) != 1 || manifests[0].Name != "faketube" {
		t.Fatalf("want 1 good manifest, got %v", manifests)
	}
	if len(errs) != 1 {
		t.Fatalf("want 1 error, got %v", errs)
	}
}

func TestParseRejectsUnknownFields(t *testing.T) {
	if _, err := Parse([]byte(`{"name":"x","binary":"b","serach":{}}`), "t.json"); err == nil {
		t.Fatal("want error on typoed field")
	}
}

func TestGetPath(t *testing.T) {
	doc := map[string]any{"a": map[string]any{"b": []any{map[string]any{"c": "hit"}}}}
	if got := get(doc, "a.b.0.c"); got != "hit" {
		t.Fatalf("got %v", got)
	}
	if got := get(doc, "a.b.5.c"); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
	if got := get(doc, "a.z"); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}
