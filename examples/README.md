# agen examples

Committed generated packages for **real public AsyncAPI documents** (own `go.mod`, like ogen's examples setup):
regenerating them on every change catches ecosystem drift — real documents exercise shapes our own test corpus
doesn't.

Each directory holds the upstream document (`asyncapi.yaml`), an `agen.yml`, a `generate.go` driving
`go:generate`, and the committed `api/` package. Regenerate and test with:

```sh
cd examples
go generate ./...
go test ./...
```

The module pins the generator with a `replace` to the parent directory.

## Corpus and provenance

| Example | Source | Notes |
|---|---|---|
| `streetlights-kafka` | [asyncapi/spec-json-schemas](https://github.com/asyncapi/spec-json-schemas) `test/fixtures/asyncapi.yml` — the canonical Streetlights Kafka 3.0 document | kafka-secure servers with security schemes, components-heavy refs, channel parameters. A stray `dupa: test` test field was removed. |
| `converted-streetlights` | [asyncapi/converter-js](https://github.com/asyncapi/converter-js) `test/output/3.0.0/from-2.6.0-with-deep-local-references.yml` — official converter output of a real 2.6 document | deep cross-channel schema references and intentionally circular schemas (recursive optional fields lower to Go pointers). Demonstrates the documented 2.x → 3.x upgrade workflow. |
| `converted-params` | [asyncapi/converter-js](https://github.com/asyncapi/converter-js) `test/output/3.0.0/from-2.6.0-with-reference-parameter.yml` | channel parameters referenced from components; converter-era channel keys contain `/` and `{}` (kept verbatim in generated identifiers). |

Both upstream repositories are Apache-2.0 © the AsyncAPI Initiative. Documents are copied unmodified except
where noted; see the `LICENSE` and `NOTICE` files of the upstream repositories.

Documents that agen cannot generate yet are deliberately not included (e.g. converter outputs with external
`https://` message references or Avro `schemaFormat`) — they are future work tracked in the roadmap.

## Runnable Redis demo

`redis/` is a self-contained runnable example of the Redis runtime (agen's own demo spec, not an upstream
document): it publishes typed light commands and receives them back through a consumer group —
publish → XADD → XREADGROUP → decode → validate → handler → XACK. Start a Redis server and:

```sh
go run ./redis                # streams mode (default)
go run ./redis -mode pubsub   # fire-and-forget Pub/Sub mode
go run ./redis -n 5 -addr redis.example.io:6379
```

`redis/main_test.go` runs the same flow against an in-memory Redis (miniredis).
