# HTTP API

The server binds to `:8879` by default (`shirabe serve --addr`).
All responses are JSON unless noted.

## GET /api/query?q=

The main endpoint.
By default it answers with a server-sent event stream:

```
event: cards
data: {"type":"cards","source":"youtube","cards":[{...}]}

event: error
data: {"type":"error","source":"stooq","message":"stooq: no data"}

event: done
data: {"type":"done","timings":[{"source":"youtube","ms":412}],"errors":[...]}
```

`cards` events arrive per source as each one finishes, so fast sources render first.
`done` is always the last event and carries per-source timings plus any errors.
A comment heartbeat (`: ping`) goes out every 15 seconds.

With `&stream=0` the same result comes back as one buffered JSON object `{query, cards, errors, timings}`, cached for 60 seconds.

Queries are classified server side: URLs route to the owning source, `weather in tokyo` / `define x` / `stock TSLA` hit intent sources, `!yt`, `!wiki`, `!hn`, `!amz`, `!gr` force one source.

## GET /api/resolve?url=

Dereferences one absolute http(s) URL into cards.
Answers `{query, cards, errors, timings}`.
The first source whose hosts match the URL wins; the OpenGraph/JSON-LD page reader is the fallback for any host nobody owns.

## GET /api/sources

Lists every registered source with capabilities and availability:

```json
[{"name":"youtube","search":true,"resolve":true,"hosts":["youtube.com","youtu.be"],"intents":[],"available":true}]
```

## GET /api/suggest?q=

Typeahead.
Returns a JSON array of strings.
Backed by Wikipedia opensearch with a 2 second budget; failures return `[]`, never an error.

## GET /img?u=

Image proxy so card thumbnails never leak the browser's IP to third parties and never mix insecure content.
Only fetches `image/*`, caps at 5 MB, and refuses redirects into loopback, private, or link-local addresses after DNS resolution.
Sets `Cache-Control: public, max-age=3600` and `X-Content-Type-Options: nosniff`.

## GET /healthz

`{"status":"ok","version":"..."}`.

## Cards

Every card shares one envelope:

```json
{
  "kind": "video",
  "source": "youtube",
  "title": "...",
  "url": "https://...",
  "snippet": "...",
  "thumbnail": "https://...",
  "score": 0.9,
  "fetched_at": "2026-07-06T12:00:00Z",
  "body": { "channel": "...", "duration": "12:34" }
}
```

`kind` is one of `web`, `video`, `image`, `article`, `product`, `book`, `weather`, `chart`, `entity`, `definition`, `qa`, `post`, `repo`, `place`.
`body` is typed per kind; see `pkg/schema/schema.go` for the exact shapes.
Consumers should treat unknown kinds as `web`: the server already downgrades kinds it does not know, so this only matters for clients ahead of the server.
