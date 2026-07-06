package server

import (
	"net/http"

	"github.com/tamnd/shirabe/pkg/schema"
)

// handleDevCards feeds /dev/cards one fixture of every kind so renderers can
// be eyeballed in both themes without live sources. Dev mode only.
func (s *Server) handleDevCards(w http.ResponseWriter, r *http.Request) {
	cards := []schema.Card{
		{Kind: schema.KindEntity, Source: "wikipedia", Title: "Alan Turing", URL: "https://en.wikipedia.org/wiki/Alan_Turing",
			Thumbnail: "https://upload.wikimedia.org/wikipedia/commons/thumb/a/a1/Alan_Turing_Aged_16.jpg/250px-Alan_Turing_Aged_16.jpg",
			Body: &schema.EntityBody{
				Description: "Alan Mathison Turing was an English mathematician, computer scientist, logician, and cryptanalyst, widely considered the father of theoretical computer science.",
				Facts: []schema.Fact{
					{Label: "Born", Value: "23 June 1912, Maida Vale, London"},
					{Label: "Died", Value: "7 June 1954, Wilmslow, Cheshire"},
					{Label: "Known for", Value: "Turing machine, the Turing test, Enigma cryptanalysis"},
				},
				Attribution: "From Wikipedia, the free encyclopedia",
			}},
		{Kind: schema.KindWeather, Source: "weather", Title: "Weather in Tokyo, Japan",
			Body: &schema.WeatherBody{
				Place: "Tokyo, Japan", TempC: 27.4, Condition: "Partly cloudy", Icon: "sun-cloud",
				WindKmh: 14.2, Humidity: 68,
				Forecast: []schema.ForecastDay{
					{Date: "2026-07-06", HiC: 31, LoC: 24, Condition: "Clear sky", Icon: "sun"},
					{Date: "2026-07-07", HiC: 32, LoC: 25, Condition: "Partly cloudy", Icon: "sun-cloud"},
					{Date: "2026-07-08", HiC: 29, LoC: 24, Condition: "Rain", Icon: "rain"},
					{Date: "2026-07-09", HiC: 28, LoC: 23, Condition: "Rain showers", Icon: "rain"},
					{Date: "2026-07-10", HiC: 30, LoC: 24, Condition: "Overcast", Icon: "cloud"},
					{Date: "2026-07-11", HiC: 33, LoC: 26, Condition: "Clear sky", Icon: "sun"},
					{Date: "2026-07-12", HiC: 34, LoC: 27, Condition: "Thunderstorm", Icon: "storm"},
				},
			}},
		{Kind: schema.KindChart, Source: "stooq", Title: "AAPL 227.52 (+1.24%)", URL: "https://stooq.com/q/?s=aapl.us",
			Snippet: "Close 2026-07-03, previous 224.73, last 90 sessions",
			Body: &schema.ChartBody{
				ChartKind: "line",
				XLabels:   []string{"Mar", "", "", "Apr", "", "", "May", "", "", "Jun", "", "Jul"},
				Series: []schema.Series{{Name: "AAPL", Points: []float64{
					198, 202, 205, 199, 204, 211, 214, 209, 216, 221, 224.7, 227.5}}},
			}},
		{Kind: schema.KindVideo, Source: "youtube", Title: "Building a search engine from scratch", URL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			Thumbnail: "https://i.ytimg.com/vi/dQw4w9WgXcQ/hq720.jpg",
			Body: &schema.VideoBody{Channel: "Systems Weekly", Duration: "24:31", Views: 1284003,
				Published: "2 months ago", EmbedURL: "https://www.youtube.com/embed/dQw4w9WgXcQ"}},
		{Kind: schema.KindProduct, Source: "amazon", Title: "Anker 65W USB-C Charger, 3-Port Compact Wall Adapter", URL: "https://www.amazon.com/dp/B0000000",
			Body: &schema.ProductBody{Price: "39.99", Currency: "USD", Rating: 4.7, RatingCount: 41233,
				Availability: "In stock", Merchant: "AnkerDirect"}},
		{Kind: schema.KindBook, Source: "goodreads", Title: "The Annotated Turing", URL: "https://www.goodreads.com/book/show/2333956",
			Body: &schema.BookBody{Authors: []string{"Charles Petzold"}, Rating: 4.24, RatingCount: 2201, Year: 2008, Pages: 372}},
		{Kind: schema.KindDefinition, Source: "dictionary", Title: "serendipity",
			Body: &schema.DefinitionBody{Word: "serendipity", Phonetic: "/ˌsɛɹ.ənˈdɪp.ɪ.ti/",
				Senses: []schema.Sense{
					{PartOfSpeech: "noun", Meaning: "An unsought, unintended, and fortunate discovery.", Example: "finding it was pure serendipity"},
				}}},
		{Kind: schema.KindQA, Source: "hackernews", Title: "Show HN: I built a Google-style front end for my CLIs", URL: "https://news.ycombinator.com/item?id=1",
			Body: &schema.QABody{Question: "Show HN: I built a Google-style front end for my CLIs", Votes: 342, Comments: 128}},
		{Kind: schema.KindArticle, Source: "page", Title: "How server-sent events keep it simple", URL: "https://blog.example/sse",
			Body: &schema.ArticleBody{Author: "Mika Ito", Published: "2026-05-14", WordCount: 1850,
				Excerpt: "WebSockets are two-way; most pages only need one. Server-sent events cover the read side with plain HTTP."}},
		{Kind: schema.KindWeb, Source: "wikipedia", Title: "Search engine - Wikipedia", URL: "https://en.wikipedia.org/wiki/Search_engine",
			Snippet: "A search engine is a software system that finds web pages that match a web search.",
			Body:    &schema.WebBody{Site: "en.wikipedia.org", DisplayURL: "en.wikipedia.org > wiki > Search_engine"}},
		{Kind: schema.KindPost, Source: "x", Title: "Post by @nasa", URL: "https://x.com/nasa/status/1",
			Body: &schema.PostBody{Author: "NASA", Handle: "@nasa", Text: "The Sun never sets on science.", Likes: 20412, Reposts: 3120, Published: "2026-07-01"}},
		{Kind: schema.KindRepo, Source: "page", Title: "tamnd/shirabe", URL: "https://github.com/tamnd/shirabe",
			Body: &schema.RepoBody{Owner: "tamnd", Stars: 128, Language: "Go", Description: "One box over every source you already have."}},
		{Kind: schema.KindImage, Source: "oembed", Title: "Mount Fuji at dawn", URL: "https://photos.example/fuji",
			Thumbnail: "https://images.unsplash.com/photo-1490806843957-31f4c9a91c65?w=640",
			Body:      &schema.ImageBody{Width: 640, Height: 427, SourcePage: "https://photos.example/fuji"}},
		{Kind: schema.KindPlace, Source: "page", Title: "Shibuya Crossing", URL: "https://maps.example/shibuya",
			Body: &schema.PlaceBody{Address: "2-2-1 Dogenzaka, Shibuya, Tokyo", Lat: 35.6595, Lon: 139.7005, Rating: 4.6}},
	}
	writeJSON(w, schema.Result{Query: "dev fixtures", Cards: cards})
}
