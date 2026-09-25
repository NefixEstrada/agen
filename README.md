# agen

**agen** is to [AsyncAPI](https://www.asyncapi.com) what [ogen](https://github.com/ogen-go/ogen) is to OpenAPI: a
pure-Go, `go install`-able code generator that turns an **AsyncAPI 3.x** document into typed, validating Go code —
message types, streaming JSON codecs, validators, typed publishers and subscribers, and broker runtime bindings
(**Redis** first — Streams and Pub/Sub; Kafka, MQTT, AMQP, NATS and WebSockets are later milestones).

AsyncAPI 2.x is explicitly out of scope (3.x only): upgrade documents with the official
[`@asyncapi/converter`](https://github.com/asyncapi/converter) first.

## Status

Implemented (matching DESIGN.md milestones M0–M2):

- `cmd/agen` CLI: `agen.yml` auto-discovery, strict config decode, file/http spec loading, pretty located errors
  (line/column with source excerpt).
- `asyncapi/` raw 3.x DOM (extensions + locators) and `asyncapi/parser` semantic parser: `$ref` resolution,
  operation↔channel↔message linking, traits (must-not-override rule), bindings capture, content-type and
  schemaFormat validation, address-parameter validation. All JSON Schema parsing is delegated to ogen's
  `jsonschema` engine.
- `gen/`: IR construction over ogen's vendored schema-lowering core; emits types, JSON codecs (jx), validators,
  defaults, message envelopes, per-operation encode/decode helpers, handlers, subscriber dispatch, client
  (publisher), address builders, middleware aliases, fakes and unimplemented handlers.
- `runtime/`: broker-agnostic contract (`broker`), typed errors (`agenerrors`), vendored ogen codecs/validators,
  and the `redis` backend (Streams with consumer groups + XACK + XAUTOCLAIM reclaimer + optional DLQ, and
  Pub/Sub with pattern subscribe).
- Committed generated packages under `internal/integration/` (streetlights, orders) regenerated via `go:generate`
  and behavior-tested, including Redis round-trips against miniredis.

Also implemented: [specification extensions](docs/extensions.md) (`x-agen-*`, mirroring ogen's
`x-ogen-*` family: custom names, fields, types, time formats and pluggable validators), the
`generator.initialisms` naming option, `expand.output` (fully-dereferenced spec dump; internal
`$ref`s are inlined, external/recursive ones kept), and `operation.reply` support (the reply of a
receive operation is generated as a typed publish surface: `client.<Operation>Reply(...)` on the
reply channel; automatic reply dispatch and correlationId plumbing are future work).

Not yet implemented (see DESIGN.md roadmap): Kafka/MQTT/AMQP/NATS/WS backends (codegen is already
protocol-agnostic; Redis is the only runtime for now), Avro payloads, OTel instrumentation, official
meta-schema validation (structural validation is enforced at parse time instead, ogen-style), the optional
Modelina backend and `x-agen-redis` conventions beyond `ConsumerConfig`/`PublisherConfig`.

## Install

```sh
go install github.com/NefixEstrada/agen/cmd/agen@latest
```

## Usage

```sh
agen --config agen.yml
# or positionally: agen spec.yaml [output-dir]
```

```yaml
# agen.yml
target:
  package_name: streetlights     # default: derived from info.title
  dir: ./api
  spec: ./asyncapi.yaml          # required (file path or http(s):// URL)
parser:
  infer_types: true
  allow_remote: false
  depth_limit: 1000
  ignore_unsupported: []          # e.g. [ "schema.not", "operation.reply" ]
generator:
  initialisms: false              # apply initialism rules (Id -> ID) to identifiers
  features:
    enable:  [ publisher, subscriber, middleware, fakes, unimplemented ]
    disable: []
  filters:
    operations_regex: ""          # generate a subset
    actions: []                   # send | receive
broker:
  redis: { mode: streams }        # streams | pubsub
expand:
  output: ""                      # optional fully-dereferenced spec dump (YAML)
```

Relative paths in the config resolve against the config file directory.

### Generated API (streetlights example)

ogen-style: the generated package wires the broker runtime internally — user
code never imports `agen/runtime` (specs whose servers use a protocol without
a runtime backend yet, e.g. kafka, fall back to `WithPublisher`).

```go
// Server: consumes the spec's `receive` operations.
srv, err := streetlights.NewServer(myHandler,
	streetlights.WithAddr("redis.example.io:6379"), // default: first spec server
	streetlights.WithGroup("streetlights-app"),     // consumer group (streams mode)
	streetlights.WithMode(streetlights.ModeStreams),
)
go func() { _ = srv.Run(ctx) }() // → decode → validate → typed handler; XACK on success
<-srv.Ready()

// Client: publishes the spec's `send` operations.
client, err := streetlights.NewClient("redis.example.io:6379",
	streetlights.WithMode(streetlights.ModeStreams),
)
err = client.SendLightCommand(ctx, &streetlights.LightCommand{…})
// parameterized channels: client.SendOrderEvents(ctx, "acme", msg)

// Protocols without a runtime backend: bring your own publisher.
client, err := streetlights.NewClient("", streetlights.WithPublisher(myPublisher))
```

Low-level building blocks stay exported for custom wiring (sharing a
connection, backends without codegen support): `NewSubscriber`,
`NewSubscriberHandlers`, `NewHandler` and the `runtime/*` packages.

Multi-message channels generate a dispatch interface (`OrderEventsMessage`) with try-decode-in-order dispatch:
messages are tried in sorted spec order, so payloads matching several shapes dispatch to the first variant. Use
unique required fields or a schema-level `oneOf`/`discriminator` payload for unambiguous dispatch.

## Development

```sh
go test ./...                       # unit + integration (miniredis, no docker needed)
go generate ./internal/integration/...  # regenerate committed packages after template changes
make examples test_examples                    # regenerate + test (real public documents)
```

`examples/` is a separate module with committed generated code for real public AsyncAPI documents (the
canonical Streetlights Kafka spec plus official-converter outputs of 2.6 documents) and a runnable Redis
demo (`go run ./examples/ex_redis`); see [examples/README.md](examples/README.md).

Generated files are named `aas_*_gen.go` (AsyncAPI spec, mirroring ogen's `oas_*` convention).

See `DESIGN.md` for the full design document and `THIRD_PARTY.md` + the per-directory `UPSTREAM.md` files for
the ogen vendoring manifest.

## Attribution

agen reuses [ogen](https://github.com/ogen-go/ogen) (Apache-2.0): it depends on its public JSON-Schema engine at
build time and vendors parts of its schema-lowering core, IR and runtime packages. Generated code imports
`agen/runtime` only — never ogen.

## License

[Apache-2.0](LICENSE)
