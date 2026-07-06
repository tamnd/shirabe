package schema

import (
	"encoding/json"
	"reflect"
	"testing"
)

func sample() []Card {
	return []Card{
		{Kind: KindWeb, Source: "t", Title: "hit", URL: "https://a.example", Body: &WebBody{Site: "a.example", DisplayURL: "a.example > page"}},
		{Kind: KindVideo, Source: "t", Title: "vid", Body: &VideoBody{Channel: "ch", Duration: "3:21", Views: 42, EmbedURL: "https://e"}},
		{Kind: KindImage, Source: "t", Title: "img", Body: &ImageBody{Width: 10, Height: 20, SourcePage: "https://p"}},
		{Kind: KindArticle, Source: "t", Title: "art", Body: &ArticleBody{Author: "a", WordCount: 900, Excerpt: "x"}},
		{Kind: KindProduct, Source: "t", Title: "prod", Body: &ProductBody{Price: "9.99", Currency: "USD", Rating: 4.5, RatingCount: 12}},
		{Kind: KindBook, Source: "t", Title: "book", Body: &BookBody{Authors: []string{"a"}, Rating: 4.1, Year: 2001, Pages: 300}},
		{Kind: KindWeather, Source: "t", Title: "wx", Body: &WeatherBody{Place: "Tokyo", TempC: 21.5, Forecast: []ForecastDay{{Date: "2026-07-06", HiC: 30, LoC: 22}}}},
		{Kind: KindChart, Source: "t", Title: "ch", Body: &ChartBody{ChartKind: "line", XLabels: []string{"a", "b"}, Series: []Series{{Name: "s", Points: []float64{1, 2}}}}},
		{Kind: KindEntity, Source: "t", Title: "ent", Body: &EntityBody{Description: "d", Facts: []Fact{{Label: "Born", Value: "1912"}}}},
		{Kind: KindDefinition, Source: "t", Title: "def", Body: &DefinitionBody{Word: "w", Senses: []Sense{{PartOfSpeech: "noun", Meaning: "m"}}}},
		{Kind: KindQA, Source: "t", Title: "qa", Body: &QABody{Question: "q", Answer: "a", Votes: 3}},
		{Kind: KindPost, Source: "t", Title: "post", Body: &PostBody{Author: "a", Handle: "@a", Text: "hi"}},
		{Kind: KindRepo, Source: "t", Title: "repo", Body: &RepoBody{Owner: "o", Stars: 5, Language: "Go"}},
		{Kind: KindPlace, Source: "t", Title: "pl", Body: &PlaceBody{Address: "1 St", Lat: 1, Lon: 2}},
	}
}

func TestRoundTripEveryKind(t *testing.T) {
	for _, c := range sample() {
		b, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("%s: marshal: %v", c.Kind, err)
		}
		var got Card
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("%s: unmarshal: %v", c.Kind, err)
		}
		if got.Kind != c.Kind || got.Title != c.Title {
			t.Fatalf("%s: envelope mismatch: %+v", c.Kind, got)
		}
		if !reflect.DeepEqual(got.Body, c.Body) {
			t.Fatalf("%s: body mismatch\nwant %#v\ngot  %#v", c.Kind, c.Body, got.Body)
		}
	}
}

func TestValidateDowngradesUnknownKind(t *testing.T) {
	c := Card{Kind: "hologram", Source: "t", Title: "x", Body: map[string]any{"a": 1}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if c.Kind != KindWeb || c.Body != nil {
		t.Fatalf("want downgrade to web with nil body, got %v %v", c.Kind, c.Body)
	}
}

func TestValidateRejectsEmpty(t *testing.T) {
	if err := (&Card{Source: "t"}).Validate(); err == nil {
		t.Fatal("want error on empty kind")
	}
	if err := (&Card{Kind: KindWeb}).Validate(); err == nil {
		t.Fatal("want error on empty source")
	}
}

func TestUnknownKindBodyDecodesGeneric(t *testing.T) {
	raw := `{"kind":"hologram","source":"t","title":"x","body":{"depth":3}}`
	var c Card
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatal(err)
	}
	m, ok := c.Body.(map[string]any)
	if !ok || m["depth"] != float64(3) {
		t.Fatalf("want generic map body, got %#v", c.Body)
	}
}

func TestSortStable(t *testing.T) {
	cards := []Card{
		{Kind: KindWeb, Source: "b", Title: "y", Score: 0.5},
		{Kind: KindWeb, Source: "a", Title: "x", Score: 0.5},
		{Kind: KindWeb, Source: "c", Title: "z", Score: 0.9},
	}
	Sort(cards)
	if cards[0].Score != 0.9 || cards[1].Source != "a" || cards[2].Source != "b" {
		t.Fatalf("bad order: %+v", cards)
	}
}
