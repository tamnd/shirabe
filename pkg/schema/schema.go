// Package schema is the content model shared by every source and renderer.
// A source emits Cards, the UI renders Cards, and nothing else crosses that
// boundary. The package deliberately imports nothing outside the stdlib.
package schema

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// Kind names the shape of a card body. The set is closed here; consumers must
// treat unknown kinds as KindWeb so old servers and new UIs stay compatible.
type Kind string

const (
	KindWeb        Kind = "web"
	KindVideo      Kind = "video"
	KindImage      Kind = "image"
	KindArticle    Kind = "article"
	KindProduct    Kind = "product"
	KindBook       Kind = "book"
	KindWeather    Kind = "weather"
	KindChart      Kind = "chart"
	KindEntity     Kind = "entity"
	KindDefinition Kind = "definition"
	KindQA         Kind = "qa"
	KindPost       Kind = "post"
	KindRepo       Kind = "repo"
	KindPlace      Kind = "place"
)

// Card is the envelope. Body holds the kind-specific payload and marshals
// under the "body" key with Kind as the discriminator.
type Card struct {
	Kind      Kind      `json:"kind"`
	Source    string    `json:"source"`
	Title     string    `json:"title"`
	URL       string    `json:"url,omitempty"`
	Snippet   string    `json:"snippet,omitempty"`
	Thumbnail string    `json:"thumbnail,omitempty"`
	Score     float64   `json:"score"`
	FetchedAt time.Time `json:"fetched_at,omitzero"`
	Body      any       `json:"body,omitempty"`
}

type WebBody struct {
	DisplayURL string `json:"display_url,omitempty"`
	Site       string `json:"site,omitempty"`
}

type VideoBody struct {
	Channel   string `json:"channel,omitempty"`
	Duration  string `json:"duration,omitempty"`
	Views     int64  `json:"views,omitempty"`
	Published string `json:"published,omitempty"`
	EmbedURL  string `json:"embed_url,omitempty"`
}

type ImageBody struct {
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	SourcePage string `json:"source_page,omitempty"`
}

type ArticleBody struct {
	Author    string `json:"author,omitempty"`
	Published string `json:"published,omitempty"`
	WordCount int    `json:"word_count,omitempty"`
	Excerpt   string `json:"excerpt,omitempty"`
}

type ProductBody struct {
	Price        string  `json:"price,omitempty"`
	Currency     string  `json:"currency,omitempty"`
	Rating       float64 `json:"rating,omitempty"`
	RatingCount  int64   `json:"rating_count,omitempty"`
	Availability string  `json:"availability,omitempty"`
	Merchant     string  `json:"merchant,omitempty"`
}

type BookBody struct {
	Authors     []string `json:"authors,omitempty"`
	Rating      float64  `json:"rating,omitempty"`
	RatingCount int64    `json:"rating_count,omitempty"`
	Year        int      `json:"year,omitempty"`
	Pages       int      `json:"pages,omitempty"`
	ISBN        string   `json:"isbn,omitempty"`
}

type ForecastDay struct {
	Date      string  `json:"date"`
	HiC       float64 `json:"hi_c"`
	LoC       float64 `json:"lo_c"`
	Condition string  `json:"condition,omitempty"`
	Icon      string  `json:"icon,omitempty"`
}

type WeatherBody struct {
	Place     string        `json:"place"`
	TempC     float64       `json:"temp_c"`
	Condition string        `json:"condition,omitempty"`
	Icon      string        `json:"icon,omitempty"`
	WindKmh   float64       `json:"wind_kmh,omitempty"`
	Humidity  int           `json:"humidity,omitempty"`
	Forecast  []ForecastDay `json:"forecast,omitempty"`
}

type Series struct {
	Name   string    `json:"name,omitempty"`
	Points []float64 `json:"points"`
}

type ChartBody struct {
	ChartKind string   `json:"chart_kind"` // line, bar, spark
	XLabels   []string `json:"x_labels,omitempty"`
	Series    []Series `json:"series"`
	Unit      string   `json:"unit,omitempty"`
}

type Fact struct {
	Label string `json:"label"`
	Value string `json:"value"`
	URL   string `json:"url,omitempty"`
}

type EntityBody struct {
	Description string `json:"description,omitempty"`
	Facts       []Fact `json:"facts,omitempty"`
	Image       string `json:"image,omitempty"`
	Attribution string `json:"attribution,omitempty"`
}

type Sense struct {
	PartOfSpeech string `json:"part_of_speech,omitempty"`
	Meaning      string `json:"meaning"`
	Example      string `json:"example,omitempty"`
}

type DefinitionBody struct {
	Word     string  `json:"word"`
	Phonetic string  `json:"phonetic,omitempty"`
	Senses   []Sense `json:"senses"`
}

type QABody struct {
	Question string `json:"question,omitempty"`
	Answer   string `json:"answer,omitempty"`
	Votes    int    `json:"votes,omitempty"`
	Comments int    `json:"comments,omitempty"`
}

type PostBody struct {
	Author    string `json:"author,omitempty"`
	Handle    string `json:"handle,omitempty"`
	Text      string `json:"text,omitempty"`
	Likes     int64  `json:"likes,omitempty"`
	Reposts   int64  `json:"reposts,omitempty"`
	Published string `json:"published,omitempty"`
}

type RepoBody struct {
	Owner       string `json:"owner,omitempty"`
	Stars       int64  `json:"stars,omitempty"`
	Language    string `json:"language,omitempty"`
	Description string `json:"description,omitempty"`
}

type PlaceBody struct {
	Address string  `json:"address,omitempty"`
	Lat     float64 `json:"lat,omitempty"`
	Lon     float64 `json:"lon,omitempty"`
	Rating  float64 `json:"rating,omitempty"`
	Hours   string  `json:"hours,omitempty"`
}

// bodyFor returns a pointer to a fresh body struct for a kind, or nil when
// the kind carries no structured body beyond the envelope.
func bodyFor(k Kind) any {
	switch k {
	case KindWeb:
		return &WebBody{}
	case KindVideo:
		return &VideoBody{}
	case KindImage:
		return &ImageBody{}
	case KindArticle:
		return &ArticleBody{}
	case KindProduct:
		return &ProductBody{}
	case KindBook:
		return &BookBody{}
	case KindWeather:
		return &WeatherBody{}
	case KindChart:
		return &ChartBody{}
	case KindEntity:
		return &EntityBody{}
	case KindDefinition:
		return &DefinitionBody{}
	case KindQA:
		return &QABody{}
	case KindPost:
		return &PostBody{}
	case KindRepo:
		return &RepoBody{}
	case KindPlace:
		return &PlaceBody{}
	}
	return nil
}

// Known reports whether k is a kind this build understands.
func Known(k Kind) bool { return bodyFor(k) != nil }

// Validate checks the envelope is coherent. Unknown kinds are downgraded to
// web rather than rejected so a newer peer never breaks an older one.
func (c *Card) Validate() error {
	if c.Kind == "" {
		return fmt.Errorf("card %q: empty kind", c.Title)
	}
	if !Known(c.Kind) {
		c.Kind = KindWeb
		c.Body = nil
	}
	if c.Source == "" {
		return fmt.Errorf("card %q: empty source", c.Title)
	}
	return nil
}

// UnmarshalJSON decodes the body into the typed struct named by kind.
func (c *Card) UnmarshalJSON(data []byte) error {
	type wire struct {
		Kind      Kind            `json:"kind"`
		Source    string          `json:"source"`
		Title     string          `json:"title"`
		URL       string          `json:"url"`
		Snippet   string          `json:"snippet"`
		Thumbnail string          `json:"thumbnail"`
		Score     float64         `json:"score"`
		FetchedAt time.Time       `json:"fetched_at"`
		Body      json.RawMessage `json:"body"`
	}
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	c.Kind, c.Source, c.Title = w.Kind, w.Source, w.Title
	c.URL, c.Snippet, c.Thumbnail = w.URL, w.Snippet, w.Thumbnail
	c.Score, c.FetchedAt = w.Score, w.FetchedAt
	c.Body = nil
	if len(w.Body) == 0 || string(w.Body) == "null" {
		return nil
	}
	body := bodyFor(w.Kind)
	if body == nil {
		var generic map[string]any
		if err := json.Unmarshal(w.Body, &generic); err != nil {
			return fmt.Errorf("card %q: body: %w", w.Title, err)
		}
		c.Body = generic
		return nil
	}
	if err := json.Unmarshal(w.Body, body); err != nil {
		return fmt.Errorf("card %q: %s body: %w", w.Title, w.Kind, err)
	}
	c.Body = body
	return nil
}

// SourceError is a user-safe per-source failure attached to a Result.
type SourceError struct {
	Source  string `json:"source"`
	Message string `json:"message"`
}

// Timing records how long one source took to answer.
type Timing struct {
	Source string `json:"source"`
	Millis int64  `json:"ms"`
	Cards  int    `json:"cards"`
}

// Result is a completed query: merged cards plus per-source diagnostics.
type Result struct {
	Query   string        `json:"query"`
	Cards   []Card        `json:"cards"`
	Errors  []SourceError `json:"errors,omitempty"`
	Timings []Timing      `json:"timings,omitempty"`
}

// Sort orders cards by score descending, ties broken by source then title so
// output is stable across runs.
func Sort(cards []Card) {
	sort.SliceStable(cards, func(i, j int) bool {
		if cards[i].Score != cards[j].Score {
			return cards[i].Score > cards[j].Score
		}
		if cards[i].Source != cards[j].Source {
			return cards[i].Source < cards[j].Source
		}
		return cards[i].Title < cards[j].Title
	})
}
