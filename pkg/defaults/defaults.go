// Package defaults assembles the standard registry: native providers,
// embedded exec manifests, and user manifests from the config directory.
package defaults

import (
	"embed"
	"fmt"

	"github.com/tamnd/shirabe/pkg/source"
	"github.com/tamnd/shirabe/pkg/source/execsrc"
	"github.com/tamnd/shirabe/pkg/source/native/dictionary"
	"github.com/tamnd/shirabe/pkg/source/native/hackernews"
	"github.com/tamnd/shirabe/pkg/source/native/oembed"
	"github.com/tamnd/shirabe/pkg/source/native/page"
	"github.com/tamnd/shirabe/pkg/source/native/stooq"
	"github.com/tamnd/shirabe/pkg/source/native/weather"
	"github.com/tamnd/shirabe/pkg/source/native/wikipedia"
)

//go:embed sources.d/*.json
var builtinFS embed.FS

// Registry builds the default source set. Warnings are non-fatal problems
// worth printing at startup, e.g. a malformed user manifest.
func Registry() (*source.Registry, []error) {
	reg := source.NewRegistry()
	var warnings []error

	natives := []source.Source{
		weather.New(),
		wikipedia.New(),
		hackernews.New(),
		dictionary.New(),
		stooq.New(),
		oembed.New(),
		page.New(),
	}
	for _, s := range natives {
		if err := reg.Register(s); err != nil {
			warnings = append(warnings, err)
		}
	}

	builtin, errs := execsrc.LoadFS(builtinFS, "sources.d")
	warnings = append(warnings, errs...)
	for _, m := range builtin {
		if err := reg.Register(execsrc.New(m)); err != nil {
			warnings = append(warnings, err)
		}
	}

	// User manifests shadow builtins of the same name.
	if dir := execsrc.UserDir(); dir != "" {
		user, errs := execsrc.LoadDir(dir)
		for _, err := range errs {
			warnings = append(warnings, fmt.Errorf("%s: %w", dir, err))
		}
		for _, m := range user {
			reg.Replace(execsrc.New(m))
		}
	}
	return reg, warnings
}
