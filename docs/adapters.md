# Exec adapters

An exec adapter turns any CLI that prints JSON into a shirabe source.
The adapter is described entirely by a manifest file; there is no Go to write.

Manifests are loaded from two places, in order:

1. Built-ins embedded in the binary (`pkg/defaults/sources.d/`).
2. User manifests in `~/.config/shirabe/sources.d/*.json` (on macOS `~/Library/Application Support/shirabe/sources.d/`, per `os.UserConfigDir`).

A user manifest with the same `name` replaces the built-in one.
A bad manifest is reported as a warning at startup and skipped; the rest still load.
A manifest whose binary is not on PATH stays registered but shows `missing` in `shirabe sources` and is skipped at query time.

## Manifest shape

```json
{
  "name": "youtube",
  "binary": "ytb",
  "priority": 50,
  "hosts": ["youtube.com", "youtu.be"],
  "intents": [],
  "search": { ... },
  "resolve": { ... }
}
```

| Field | Required | Meaning |
|---|---|---|
| `name` | yes | unique source name; also the bang target and the `source` field on cards |
| `binary` | yes | executable looked up on PATH |
| `priority` | no | ordering hint, lower runs earlier; natives use 0-150, page fallback is 200 |
| `hosts` | for resolve | host suffixes this source owns; `youtube.com` also matches `m.youtube.com` |
| `intents` | no | intent words this source answers (`weather`, `define`, `stock`) |
| `search` | one of | op run for text queries |
| `resolve` | one of | op run when a pasted URL matches `hosts` |

Unknown fields are rejected, so typos fail loudly instead of silently doing nothing.

## Ops

```json
{
  "args": ["video", "{url}", "-o", "jsonl", "-q"],
  "output": "jsonl",
  "items": "",
  "kind": "video",
  "map": { "title": "title", "url": "url" },
  "id_pattern": "v=([A-Za-z0-9_-]{11})",
  "timeout_ms": 5000
}
```

| Field | Meaning |
|---|---|
| `args` | argv after the binary name; placeholders `{query}`, `{url}`, `{id}`, `{n}` (result limit) |
| `output` | stdout shape: `jsonl` (default), `json` (one object), `array` (top-level array) |
| `items` | for `json` output, dot path to the record array, e.g. `data.items` |
| `kind` | the card kind every record becomes; must be a known kind |
| `map` | card field to dot path in the record |
| `id_pattern` | resolve only: regex with one capture group applied to the URL to fill `{id}` |
| `timeout_ms` | overrides the per-source deadline |

Map keys are the card envelope fields `title`, `url`, `snippet`, `thumbnail`, `score`, plus `body.<name>` for anything kind-specific.
At least one of `title` or `url` must be mapped.
Dot paths walk nested objects and arrays: `images.0` is the first element of `images`.

## Execution model

The binary runs with an argv vector, never through a shell, so nothing in a query can be injected.
stdout is capped at 8 MB and stderr at 4 KB; the first stderr line becomes the error message shown in the UI's footer.
The process inherits the per-source deadline (5 s by default) and gets 2 s to exit after the deadline before it is killed.

## Trying a manifest

```sh
shirabe sources                 # is it registered and available?
shirabe search "!yourname test" # force just this source
shirabe resolve "https://..."   # exercise the resolve op
```

Run `shirabe serve --dev` and watch the terminal: adapter errors surface as `error` events with the source name.
