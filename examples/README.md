# agen examples

Committed generated code for **real public AsyncAPI documents**, mirroring ogen's examples organization:

- the documents live in the main module at `_testdata/examples/`;
- a single [generate.go](generate.go) at this module's root drives every generation with explicit
  `go:generate` lines (`--clean`, `--config config/<variant>.yml`, `--target ex_<name>`);
- shared configs live in [config/](config/) (feature toggles only, ogen-style);
- each `ex_<name>/` holds the generated package (package name `api` by default, like ogen);
- this is a separate Go module (own `go.mod`, `replace` to the parent, `tool` directive pinning
  `cmd/agen`), so the generator never leaks into users' dependency graphs.

Regenerate, build and test (or use the root Makefile targets):

```sh
cd examples
go generate        # or: make examples
go build ./...     # compiling real documents is the drift check
go test ./...      # runnable demo tests (redis)
```

## Corpus and provenance

| Example | Source | Notes |
|---|---|---|
| `ex_streetlights_kafka` | [asyncapi/spec-json-schemas](https://github.com/asyncapi/spec-json-schemas) `test/fixtures/asyncapi.yml` — the canonical Streetlights Kafka 3.0 document | kafka-secure servers with security schemes, components-heavy refs, channel parameters. A stray `dupa: test` test field was removed. |
| `ex_converted_streetlights` | [asyncapi/converter-js](https://github.com/asyncapi/converter-js) `test/output/3.0.0/from-2.6.0-with-deep-local-references.yml` — official converter output of a real 2.6 document | deep cross-channel schema references and intentionally circular schemas (recursive optional fields lower to Go pointers). Demonstrates the documented 2.x → 3.x upgrade workflow. |
| `ex_converted_params` | [asyncapi/converter-js](https://github.com/asyncapi/converter-js) `test/output/3.0.0/from-2.6.0-with-reference-parameter.yml` | channel parameters referenced from components; converter-era channel keys contain `/` and `{}` (kept verbatim in generated identifiers). |

Both upstream repositories are Apache-2.0 © the AsyncAPI Initiative. Documents are copied unmodified except
where noted; see the `LICENSE` and `NOTICE` files of the upstream repositories.

Documents that agen cannot generate yet are deliberately not included (e.g. converter outputs with external
`https://` message references or Avro `schemaFormat`) — they are future work tracked in the roadmap.

## Runnable Redis demo

`redis/` is a self-contained runnable example of the Redis runtime (agen's own demo spec in
`_testdata/examples/redis-demo.yaml`, not an upstream document): it publishes typed light commands and
receives them back through a consumer group — publish → XADD → XREADGROUP → decode → validate → handler →
XACK. Start a Redis server and:

```sh
go run ./redis                # streams mode (default)
go run ./redis -mode pubsub   # fire-and-forget Pub/Sub mode
go run ./redis -n 5 -addr redis.example.io:6379
```

`redis/main_test.go` runs the same flow against an in-memory Redis (miniredis).
