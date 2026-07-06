// Package dictionary answers "define <word>" via dictionaryapi.dev, keyless.
package dictionary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
	"github.com/tamnd/shirabe/pkg/source/native/httpx"
)

type Source struct {
	Client *httpx.Client
	Base   string
}

func New() *Source {
	return &Source{Client: httpx.New(), Base: "https://api.dictionaryapi.dev"}
}

func (s *Source) Name() string  { return "dictionary" }
func (s *Source) Priority() int { return 10 }

func (s *Source) Caps() source.Caps {
	return source.Caps{Intents: []string{"define"}}
}

func (s *Source) Resolve(ctx context.Context, u *url.URL) ([]schema.Card, error) {
	return nil, source.ErrNotHandled
}

func (s *Source) Search(ctx context.Context, q source.Query) ([]schema.Card, error) {
	if q.Intent != "define" || q.Arg == "" {
		return nil, source.ErrNotHandled
	}
	raw, err := s.Client.Get(ctx, fmt.Sprintf("%s/api/v2/entries/en/%s", s.Base, url.PathEscape(q.Arg)))
	if err != nil {
		return nil, err
	}
	var entries []struct {
		Word     string `json:"word"`
		Phonetic string `json:"phonetic"`
		Meanings []struct {
			PartOfSpeech string `json:"partOfSpeech"`
			Definitions  []struct {
				Definition string `json:"definition"`
				Example    string `json:"example"`
			} `json:"definitions"`
		} `json:"meanings"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil || len(entries) == 0 {
		return nil, source.ErrNotHandled
	}
	e := entries[0]
	body := &schema.DefinitionBody{Word: e.Word, Phonetic: e.Phonetic}
	for _, m := range e.Meanings {
		for i, d := range m.Definitions {
			if i >= 3 {
				break
			}
			body.Senses = append(body.Senses, schema.Sense{
				PartOfSpeech: m.PartOfSpeech, Meaning: d.Definition, Example: d.Example,
			})
		}
	}
	if len(body.Senses) == 0 {
		return nil, source.ErrNotHandled
	}
	return []schema.Card{{
		Kind: schema.KindDefinition, Source: s.Name(),
		Title: e.Word, FetchedAt: time.Now(), Body: body,
	}}, nil
}
