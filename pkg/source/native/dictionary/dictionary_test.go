package dictionary

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
)

const body = `[{"word":"serendipity","phonetic":"/ˌsɛɹ.ənˈdɪp.ɪ.ti/","meanings":[
	{"partOfSpeech":"noun","definitions":[{"definition":"An unsought fortunate discovery.","example":"pure serendipity"}]}
]}]`

func TestDefine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	s := New()
	s.Base = srv.URL
	cards, err := s.Search(context.Background(), source.Query{Intent: "define", Arg: "serendipity"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || cards[0].Kind != schema.KindDefinition {
		t.Fatalf("bad cards: %+v", cards)
	}
	d := cards[0].Body.(*schema.DefinitionBody)
	if d.Word != "serendipity" || len(d.Senses) != 1 || d.Senses[0].PartOfSpeech != "noun" {
		t.Fatalf("bad body: %+v", d)
	}
	if _, err := s.Search(context.Background(), source.Query{Raw: "serendipity"}); err != source.ErrNotHandled {
		t.Fatalf("want ErrNotHandled without intent, got %v", err)
	}
}
