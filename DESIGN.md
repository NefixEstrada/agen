# agen — Design Document

**Status:** Draft v1 (study phase) · **Date:** 2026-09-23
**Sources:** hands-on study of [ogen](https://github.com/ogen-go/ogen) (code-level, see `docs/research/ogen.md`), [Modelina](https://github.com/asyncapi/modelina) v5.10.1 (executed against AsyncAPI 2.6 & 3.0, see `docs/research/modelina.md`), and the AsyncAPI ecosystem (see `docs/research/asyncapi-ecosystem.md`).

> **TL;DR** — `agen` is to [AsyncAPI](https://www.asyncapi.com) what [ogen](https://github.com/ogen-go/ogen) is to OpenAPI: a pure-Go, `go install`-able code generator that turns an AsyncAPI 3.x document into typed, validating Go code — message types, streaming JSON codecs, validators, typed publishers and subscribers, and broker runtime bindings (**Redis first** — Streams and Pub/Sub; then Kafka, MQTT, AMQP, NATS and WebSockets). AsyncAPI 2.x is explicitly out of scope (3.x only). The strategy is **maximum reuse**: agen *depends* on ogen's public JSON-Schema engine (build time) and *vendors* ogen's schema-lowering core, IR and runtime validation packages (with attribution); everything AsyncAPI-specific — document model, send/receive semantics, protocol bindings, pub/sub stubs, broker runtimes — is new code that mirrors ogen's architecture patterns. Modelina is studied and **rejected as a core component** (Node-only, weakest-in-class Go output) but kept as a design reference and a possible optional backend.

---

## 1. Motivation and goals

Event-driven APIs (Kafka, MQTT, RabbitMQ…) lack what OpenAPI got with ogen: a generator that produces *idiomatic, typed, validating* Go from a spec, with no template mush and no runtime reflection. Today's Go landscape confirms the gap:

- The official AsyncAPI **Go parser is archived** (asyncapi-archived-repos/parser-go, archived 2025, 2.x only) and the official generator's **Go template is archived** (go-watermill-template). No maintained Go template exists.
- The most serious Go generator, [lerenn/asyncapi-codegen](https://github.com/lerenn/asyncapi-codegen) (161★, active), supports **only Kafka + NATS**, emits a single file, and has documented validation gaps.
- [bdragon300/go-asyncapi](https://github.com/bdragon300/go-asyncapi) is the closest in ambition (many protocols) but is explicitly unstable, unreleased, and template-driven.
- **ogen has zero AsyncAPI plans** (no issues/discussions touch it), so there is no collision with upstream — and its engine is exactly the reusable core we need.

### Goals

1. Input: **AsyncAPI 3.0.x / 3.1.0 documents only** (JSON or YAML). AsyncAPI 2.x is **out of scope** — documents must be upgraded to 3.x first (the official `@asyncapi/converter` or Studio do this; there is no maintained Go converter).
2. Output: a Go package with types, codecs, validators, and **typed publisher/subscriber code** per operation, plus a small runtime library with per-protocol broker bindings.
3. Pure Go toolchain: single binary, `go install github.com/agen-go/agen/cmd/agen@latest`, `go:generate`-friendly. **No Node, no Docker, no protoc** at generation time.
4. ogen-grade ergonomics: precise line/column error reporting, strict config, feature flags, `ignore_unsupported` escape hatch, generated code that reads hand-written.
5. Reuse-first ("be the glue"): every generic piece of machinery (JSON Schema → Go types, validation codegen, naming, formatting) comes from ogen, vendored or imported; agen only writes genuinely new logic.

### Non-goals (for v1)

- AsyncAPI 2.x / 1.x input — no internal transform; upgrade documents upstream with the official JS converter.
- Polyglot output (Go only — Modelina exists if you need 12 languages).
- Generating broker infrastructure (topics, queues) — bindings are *read*, not provisioned. (A future `agen apply` is conceivable; out of scope.)
- Schema-registry integration (Confluent SR, Avro wire codecs) — later milestone, opt-in.
- A AsyncAPI document *linter* — [vacuum](https://github.com/daveshanley/vacuum) already does this; possible future integration only.
- Spec-from-code (reverse direction) — see [swaggest/go-asyncapi](https://github.com/swaggest/go-asyncapi) / [polanski13/asyngo](https://github.com/polanski13/asyngo) for that.

---

## 2. Prior art and the reuse decision

### 2.1 ogen (the foundation)

ogen's pipeline is: raw spec (`ogen.Spec`, YAML with locators) → semantic parse (`openapi/parser` → `openapi.API`, delegating **all** schema work to `jsonschema.Parser`) → IR construction (`gen` + `gen/ir`) → emission (`text/template` + goimports). Crucially:

- **`jsonschema/` is a public package with zero OpenAPI dependencies** — an OpenAPI-3.x-flavored JSON Schema dialect engine that covers the AsyncAPI Schema Object (a draft-07 superset) almost entirely: `properties/required`, `allOf` (flattened), `oneOf/anyOf` (sum types), `enum/const`, full validator vocabulary, formats, external `$ref`s (file/http), recursion detection, type inference. agen can **import it as a library**.
- The **schema→IR lowering core** (`gen/schema_gen*.go`, `tstorage.go`, `names.go`, `generics.go`, `gen_equality*.go`, …) is separable from `gen`'s HTTP machinery but lives in the same package and `gen/ir` back-references `openapi.Operation` — this part must be **copied and adapted**.
- The **runtime packages** the generated code imports (`validate`, `json` (jx codecs), `ogenregex`, `conv`) are small and self-contained — copy them so generated code never depends on ogen.
- `cmd/jschemagen` proves the schema pipeline runs standalone; it is the skeleton for `cmd/agen`.

License note: ogen is **Apache-2.0**. agen will also be **Apache-2.0**, which makes vendoring friction-free; copied files keep upstream headers and `THIRD_PARTY.md` records the origin commit.

### 2.2 Modelina (studied, rejected as core)

Verified by running v5.10.1 against AsyncAPI 2.6/3.0 documents (`docs/research/modelina.md` has the transcripts):

- It is the AsyncAPI Initiative's official *model* generator (Node ≥ 18), inputs AsyncAPI 2.0–3.0 plus OpenAPI/Avro/RAML/XSD, outputs 12 languages.
- Its Go output is the thinnest of all languages: structs + json tags only; **no `time.Time`** (issue #2619), **headers are never processed** (issue #2482), inline payloads get `AnonymousSchema_N` names (message `name` is ignored), a Gin `binding:"required"` tag is unconditionally baked in, `AdditionalProperties` field on every struct, enums as `uint`+`iota`, **no validation code at all**, output not gofmt'ed.
- It cannot be used from Go except by shelling out to `npx @asyncapi/modelina-cli`, whose Go surface is exactly two flags (`--goIncludeTags`, `--goIncludeComments`).

**Decision:** Modelina is (a) an architectural reference — its input-processor → meta-model → constrain → render-preset layering, naming-constraint pipeline, and multi-`schemaFormat` dispatch inform agen's design; (b) a possible *optional* `--models-backend=modelina` escape hatch for teams wanting multi-language models (requires Node; explicitly documented as such, never the default). The core path is Go-native.

### 2.3 Alternatives considered

| Alternative | Verdict | Why |
|---|---|---|
| Wrap Modelina via its CLI | ✗ | Node runtime inside a Go toolchain; Go output gaps (§2.2) would force post-processing that erases the benefit. |
| Map AsyncAPI → synthetic OpenAPI and drive ogen's public `gen.NewGenerator` end-to-end | ✗ as primary | Types/codecs/validators fall out for free, but it also generates HTTP server/client scaffolding around fake paths — dead weight in the user's package; bindings and pub/sub semantics don't fit. **Kept as a 1-day spike** to de-risk the lowering port (M1). |
| Build on lerenn/asyncapi-codegen or bdragon300/go-asyncapi | ✗ | Different architecture (single-file / template-driven); Kafka+NATS only vs our multi-protocol goal; would inherit their gaps rather than ogen's maturity. |
| From scratch, no ogen | ✗ | Reimplements a battle-tested JSON Schema engine — the opposite of this project's goal. |

---

## 3. What agen generates (the contract)

Worked example — Streetlights API (AsyncAPI 3.0, Redis), one receive and one send operation:

```yaml
asyncapi: 3.0.0
info: { title: Streetlights, version: 1.0.0 }
servers:
  production:
    host: redis.example.io:6379
    protocol: redis
channels:
  lightMeasured:
    address: 'streetlights.{streetlightId}.light'
    messages:
      lightMeasured:
        payload: { $ref: '#/components/schemas/lightMeasuredPayload' }
        headers: { $ref: '#/components/schemas/lightMeasuredHeaders' }
        contentType: application/json
operations:
  receiveLightMeasurement:
    action: receive                      # the APP consumes
    channel: { $ref: '#/channels/lightMeasured' }
  sendLightCommand:
    action: send                         # the APP produces
    channel: { $ref: '#/channels/lightCommand' }
```

`agen --config agen.yml` writes into `./api` (package `streetlights`), ogen-style file naming:

```go
// aas_msg_gen.go — per-message envelope: headers + payload (schema engine output)
type LightMeasured struct {
    Headers LightMeasuredHeaders `json:"-"`   // decoded from broker/protocol headers
    Payload LightMeasuredPayload `json:"-"`   // decoded from body per contentType
}

// aas_schemas_gen.go — plain ogen-quality types
type LightMeasuredPayload struct {
    Lumens int       `json:"lumens"`
    SentAt time.Time `json:"sentAt"`
}

// aas_json_gen.go — jx streaming encoders/decoders (no reflection)
// aas_validators_gen.go — Validate() per type (min/max/pattern/enums/…)
func (s *LightMeasuredPayload) Validate() error { /* … */ }

// aas_handlers_gen.go — consumer side (action: receive)
type ReceiveLightMeasurementHandler interface {
    ReceiveLightMeasurement(ctx context.Context, msg *LightMeasured) error
}

// aas_client_gen.go — producer side (action: send), ogen-style constructor
type Client struct { /* … */ }
func NewClient(addr string, opts ...ClientOption) (*Client, error) // wires the runtime publisher
func (c *Client) SendLightCommand(ctx context.Context, msg *LightCommand, opts ...PublishOption) error

// aas_server_gen.go — consumer side (action: receive), ogen-style constructor
type Server struct { /* … */ }
func NewServer(h Handler, opts ...ServerOption) (*Server, error) // wires the runtime consumer
func (s *Server) Run(ctx context.Context) error // XREADGROUP → dispatch → typed handler; XACK on success

// aas_subscriber_gen.go — wiring: raw incoming → decode → validate → dispatch (with middlewares)
type Subscriber struct { /* … */ }
func (s *Subscriber) Dispatch(ctx context.Context, raw runtime.Incoming) error

// aas_servers_gen.go — per-spec-server address constants (serverConst)
// aas_operations_gen.go — OperationName constants, one per operation
// aas_cfg_gen.go — addresses (with channel-parameter templating), content types,
//                  option machinery (ServerOption/ClientOption, ogen-style)
// aas_middleware_gen.go, aas_fakes_gen.go, aas_unimplemented_gen.go — ogen-equivalents
```

User code (the part agen never touches), ogen-style — the generated package
wires the broker runtime internally, so user code imports only the generated
package (the runtime is imported by generated code, exactly like ogen):

```go
srv, err := streetlights.NewServer(myHandler,
    streetlights.WithAddr("redis.example.io:6379"), // default: first spec server
    streetlights.WithGroup("streetlights-app"),     // consumer group for XREADGROUP (§6.5)
    streetlights.WithMode(streetlights.ModeStreams),
)
go func() { _ = srv.Run(ctx) }()
<-srv.Ready()

client, err := streetlights.NewClient("redis.example.io:6379",
    streetlights.WithMode(streetlights.ModeStreams),
)
err = client.SendLightCommand(ctx, &streetlights.LightCommand{…})
```

Unlike ogen's HTTP `Server` (an `http.Handler` driven by `net/http`), a broker
server has no stdlib driver, so the generated `Server` exposes `Run(ctx)`
instead of `ServeHTTP`. Specs whose servers use a protocol without a runtime
backend yet (e.g. kafka) generate no server wiring, and their client requires
`WithPublisher` (bring your own runtime publisher); the low-level
`NewSubscriber`/`NewHandler` building blocks stay exported for custom wiring.

Key semantic rules baked into codegen:

- `action: send` ⇒ publisher/client method; `action: receive` ⇒ handler/subscriber interface. **Always from the application's perspective** (3.x semantics — agen never reads 2.x documents).
- Multi-message operations (a channel with N messages and no discriminator) generate an ogen-style **sum interface** (`type LightMeasuredMessage interface{ lightMeasuredMessage() }`) with type-based or unique-field dispatch; ambiguous shapes fall back to "try-decode-in-order" with a generated error.
- Channel address `{param}` templates generate address-builder functions (`BuildLightMeasuredAddress(streetlightId string) string`) with parameter validation (enum/default per spec).
- Protocol binding fields are consumed where they affect types or addressing (e.g. Kafka `key` schema ⇒ typed key encoder; AMQP `is: queue|routingKey` ⇒ constructor flavor; MQTT `qos/retain` ⇒ publish options). Everything else is surfaced as typed config, not enforced. The official **Redis binding is an empty stub** — agen's Redis mapping lives in §6.5.

---

## 4. Architecture

### 4.1 Pipeline

Mirrors ogen stage-for-stage; new stages marked ★.

```
asyncapi.yaml / .json (file | http(s)://)
   │  load; YAML nodes keep locators (go-faster/yaml)              [adapted from cmd/ogen]
   ▼
★ asyncapi.Spec — raw 3.x DOM, 1:1 with the spec, extensions map +
 │               location.Locator on every object                   [modeled on ogen root Spec]
 │  optional meta-schema validation (embedded official schemas)
 ▼
★ asyncapi/parser — semantic parse → asyncapi.API:
 │   · resolve $ref (JSON Reference semantics) via ogen jsonpointer
 │   · operation↔channel↔message linking, traits (must-not-override),
 │   · bindings normalization per protocol, schemaFormat dispatch,
 │   · ALL schema parsing delegated to ogen jsonschema.Parser        [modeled on openapi/parser]
 ▼
★ gen — IR construction:
 │   per operation/message → Generator.generateSchema
 ▼
★ internal/lowering — schema → ir.Type lowering
 │   (copied from ogen gen: schema_gen*, tstorage, names, generics…) → ir.Types
 ▼
★ gen.WriteSource — text/template (copied schema/json/validators/defaults
 │   templates + new pub/sub/channel/broker templates) → goimports → genfs
 ▼
generated package (imports agen/runtime only, never ogen)
```

### 4.2 Package layout

```
agen/
├── cmd/agen/                  # CLI (loading, config discovery, pretty errors)
├── asyncapi/                  # ★ raw DOM: Spec, Channel, Operation, Message, Server, Bindings… (+ Extensions, Locator)
│   └── parser/                # ★ semantic parser → asyncapi.API (delegates to jsonschema)
├── gen/                       # ★ orchestration: NewGenerator(spec, opts), WriteSource (mirrors ogen/gen)
├── internal/
│   ├── ir/                    # ✚ copied from ogen gen/ir, HTTP back-refs replaced by asyncapi models
│   ├── lowering/              # ✚ copied from ogen gen schema core (files keep upstream names for diffability)
│   ├── naming/ bitset/ …      # ✚ copied from ogen internal/* as needed by lowering
│   └── _template/             # ✚ copied template subset + ★ new: client, subscriber, middleware, broker cfg
├── runtime/                   # ★ what generated code imports (agen's public runtime)
│   ├── broker                 #   Incoming/Outgoing message, Publisher, Dispatcher, middleware types
│   ├── codec                  # ✚ from ogen json/ (jx codecs: time, uuid, decimal, ip…)
│   ├── validate/ ogenregex/ conv/  # ✚ from ogen (validation helpers, regex, conversions)
│   ├── agenerrors             # ★ typed decode/validate errors (modeled on ogenerrors, no net/http)
│   ├── redis/                 # ★ go-redis backend (M2): Streams + Pub/Sub (§6.5)
│   ├── kafka/                 # ★ franz-go backend (M3)
│   └── mqtt/ amqp/ nats/ ws/  # ★ later milestones (paho, amqp091-go, nats.go, coder/websocket)
├── schemas/                   # separate go.mod: official AsyncAPI meta-schemas (like ogen/schemas)
├── examples/                  # separate go.mod: committed generated code for public specs (like ogen/examples)
└── _testdata/                 # positive/, negative/, location/, bindings/ spec corpus + per-case agen.yml
```

### 4.3 Reuse manifest (the "pega" plan)

| ogen source | Disposition | Destination |
|---|---|---|
| `jsonschema/`, `jsonpointer/`, `location/` | **depend** (public, OpenAPI-free) | go.mod dependency |
| `gen/genfs` | **depend** | go.mod dependency |
| `gen/schema_gen*.go`, `tstorage.go`, `names.go`, `generics.go`, `gen_equality*.go`, `genctx.go` | **copy + adapt** | `internal/lowering/` |
| `gen/ir` (type system, minus `operation.go` HTTP parts, `params.go`, `security.go` back-refs) | **copy + adapt** | `internal/ir/` |
| `gen/_template/{schema,json,validators,defaults,faker,godoc,globals,cfg}.tmpl` | **copy subset** | `internal/_template/` |
| `validate/`, `json/`, `ogenregex/`, `conv/` | **copy** | `runtime/{validate,codec,ogenregex,conv}/` |
| `internal/{naming,bitset,xmaps,xslices,urlpath}` | **copy as needed** | `internal/…` |
| `uri/` (header parameter codecs) | copy parts (M3+) | `runtime/` |
| `openapi/parser` structure, ref-caching, error strategy | **reference** | pattern for `asyncapi/parser` |
| `middleware/`, `ogenerrors/`, `otelogen/` | **reference** | agen gets broker-shaped equivalents |
| `http/`, `sse/`, router, `gen_operation*`, security gen | not used | — |

Copy hygiene rules: copied files keep their Apache-2.0 headers; each vendored dir gets a `UPSTREAM.md` noting the ogen commit and local deltas; file names and internal structure stay identical to upstream where possible so periodic re-diffs (`git diff --no-index`) stay mechanical. ogen itself ends up a **build-time-only** dependency — user applications never see it in their dependency graph.

Wrapped behind a thin internal facade (`internal/schema` around `jsonschema.Parser`) so ogen upgrades touch one file.

### 4.4 AsyncAPI semantics the parser must own

- **$ref discipline**: plain JSON Reference (per spec, *not* JSON-Schema `$id` chains); internal pointers, relative files, and `http(s)://` via ogen's `ExternalResolver`; ref-only links (`operation.channel`, `channel.servers`); recursion detection via `jsonpointer.ResolveCtx`.
- **Traits**: merged at parse time; a trait overriding an existing property is an error (3.0 rule).
- **schemaFormat dispatch**: default AsyncAPI dialect (draft-07 superset → ogen `jsonschema`); `application/vnd.apache.avro;version=1.9.0` etc. are recognized and, until Avro support lands, produce a clear unsupported error (not a silent skip).
- **Dialect gaps**: keywords ogen's engine lacks (e.g. `not`, `$schema`, `$defs`) produce located `ErrUnsupported` errors, overridable per-keyword via `parser.ignore_unsupported` — the analogue of ogen's beloved `ignore_not_implemented`.
- **`defaultContentType`** (default `application/json`) drives codec choice per message; non-JSON content types are validated against a known-codec table and error clearly otherwise.

---

## 5. Runtime design (`agen/runtime`)

The generated package is **broker-agnostic at its core** (decode → validate → dispatch is identical everywhere); only thin constructors are protocol-aware, mirroring how ogen generates `net/http` code.

```go
// runtime/broker
type Incoming interface {
    Topic() string
    Key() []byte
    ContentType() string
    Header(k string) string
    Body() []byte
    Ack() error
    Nack(requeue bool) error
}
type Outgoing struct {
    Topic, Key, ContentType string
    Headers map[string]string
    Body    []byte
}
type Publisher interface {
    Publish(ctx context.Context, out Outgoing) error
    Close(ctx context.Context) error
}
```

- **Backends** implement `Publisher` + consumer loops that call the generated `Subscriber.Dispatch`:
  `runtime/redis` (go-redis; Streams default + Pub/Sub opt-in per §6.5, ACL/TLS from server `security` — **M2**), `runtime/kafka` (franz-go; SASL/SSL — M3), `mqtt` (paho v3, paho.golang for v5), `amqp` (amqp091-go; exchange/queue/routingKey from bindings), `nats` (nats.go, JetStream, queue groups), `ws` (coder/websocket).
- **Errors**: `runtime/agenerrors` — `DecodeMessageError{Operation, Message, Err}` etc.; no `net/http` coupling.
- **Telemetry**: optional OTel spans/metrics per operation (pattern from ogen's `otelogen`, feature-gated).
- **Fakes**: `runtime/fake` capture-publisher + handler harness for unit tests, referenced by generated `_test` helpers (feature-gated, like ogen's `debug/example_tests`).

---

## 6. AsyncAPI scope

### 6.1 Versions

| Version | Support | Notes |
|---|---|---|
| 3.0.0 / 3.0.1 | ✅ primary target | |
| 3.1.0 | ✅ accepted | Delta is the ROS2 binding; ROS2 operations error as unsupported-protocol until someone needs it. |
| 2.x, 1.x | ✗ out of scope | No internal transform. Upgrade documents with the official `@asyncapi/converter` (JS) or Studio before feeding them to agen. |

### 6.2 Schema dialect

AsyncAPI Schema Object (draft-07 superset) + AsyncAPI extras: `discriminator` (→ ogen sum-type discrimination), `externalDocs`, `deprecated`, type-conforming `default`, boolean schemas, extra `format` values (mapped where ogen has types; unknown formats warn and fall back). `nullable` is **not** an AsyncAPI keyword (verified) — OpenAPI-style `nullable` is accepted leniently (ogen's engine already understands it) but not documented.

### 6.3 Protocol bindings by milestone

| Protocol | Milestone | Fields consumed |
|---|---|---|
| redis 0.1.0 | **M2 — first backend** | Official binding is an **empty stub** (all objects "reserved") — agen defines its own conventions (§6.5): `channel.address` = stream/pubsub-channel name, group/mode via config or `x-agen-redis`. |
| kafka 0.5.0 | M3 | channel `topic` (vs `address`), `partitions` (config surface), operation `groupId`/`clientId`, message `key` (→ typed key), `schemaRegistryUrl` (config only) |
| mqtt 0.2.0 / mqtt5 | M4 | server `clientId`, `cleanSession`, `lastWill`; operation `qos`, `retain`; v5 `correlationData`, `responseTopic` |
| amqp 0.3.0 | M4 | channel `is`, `exchange`/`queue` (declare-hints surfaced as config), operation `deliveryMode`, `priority`, `ack` |
| nats 0.1.0 | M4 | operation `queue` |
| websockets 0.1.0 | M4 | channel `method`, `query`, `headers`; message `headers` |
| googlepubsub, sns, sqs, solace, ibmmq, pulsar, http, jms… | backlog | explicit unsupported-protocol error with pointer to roadmap |
| amqp1, mercure, stomp, ros2 | backlog | official binding definitions are empty stubs — nothing to consume beyond `address`+`host` anyway |

Unknown `bindingVersion` ⇒ warning + best-effort; empty bindings ⇒ fallback to `server.host` + `channel.address` (this is what most stub bindings amount to).

### 6.4 Security

Parsed (`security` arrays on servers/operations) and lowered into typed config (SASL/SCRAM, TLS, mTLS, api-key-ish for MQTT). Enforcement is delegated to backend options (franz-go SASL, paho credentials, AMQP vhost/TLS). No OAuth2 flows in v1 (they barely make sense outside HTTP; revisit with request/reply work).

### 6.5 Redis conventions (agen-specific)

The official Redis binding (0.1.0) defines **no fields** — every object is "reserved". Until the AsyncAPI bindings project defines real ones, agen fixes its own mapping, deliberately confined to config values and `x-agen-redis` extensions so a future official binding can be adopted without touching user specs:

- **Addressing**: `channel.address` is the Redis **stream** name (`streams` mode, default) or the **pub/sub channel** name (`pubsub` mode). Address templates (`{param}`) are filled from typed publish parameters / consumer patterns (`PSUBSCRIBE` in `pubsub` mode).
- **Mode `streams` (default)** — the durable, at-least-once mapping:
  - `send` operation ⇒ `XADD <address> * payload <bytes> content_type <ct> h.<name> <value>` — body in the `payload` field, each message header as an `h.<name>` field, message ID auto-generated by Redis.
  - `receive` operation ⇒ `XREADGROUP GROUP <group> <consumer> ... STREAMS <address> >` with the group auto-created (`MKSTREAM`); group name from `x-agen-redis.group` or `ConsumerConfig.Group`.
  - `Incoming.Ack()` ⇒ `XACK`. A handler failure leaves the entry **pending**; a reclaimer loop (`XAUTOCLAIM`, idle/min-retry configurable) redelivers up to `max_deliveries`, then (opt-in) dead-letters the entry to `<address>.dlq`.
- **Mode `pubsub`** (opt-in): `send` ⇒ `PUBLISH`, `receive` ⇒ `SUBSCRIBE`/`PSUBSCRIBE`. Fire-and-forget — `Ack()`/`Nack()` are no-ops, no groups, no persistence; the generated code and docs say so loudly.
- **Security**: server `security` maps `userPassword` → Redis ACL username/password, `X509`/`scram256/512` → TLS; host/port from `server.host`.
- agen invents no other Redis key names (the `.dlq` suffix is opt-in config, not a default).

---

## 7. CLI and configuration

```
go install github.com/agen-go/agen/cmd/agen@latest
//go:generate go run github.com/agen-go/agen/cmd/agen --config agen.yml
```

- Auto-discovers `agen.yml | agen.yaml | .agen.yml | .agen.yaml` (ogen behavior); strict `KnownFields(true)` decode — typos fail loudly.
- `agen` prints ogen-style located errors (colored line/column excerpts via `location.PrintPrettyError`).

```yaml
# agen.yml
target:
  package_name: streetlights     # default: derived from info.title
  dir: ./api
parser:
  infer_types: true
  allow_remote: false
  depth_limit: 1000
  ignore_unsupported: []          # e.g. [ "schema.not", "bindingVersion" ]
generator:
  features:
    enable:  [ publisher, subscriber, middleware, fakes, otel ]  # disable_all supported
    disable: []
  filters:
    operations_regex: ""          # generate a subset
    actions: []                   # send | receive
  initialisms: [ inherit ]
broker:
  redis: { module: github.com/redis/go-redis/v9, mode: streams }   # mode: streams | pubsub (§6.5)
expand:
  output: ""                      # optional fully-dereferenced spec dump (like ogen)
```

Extensions: `x-agen-name`, `x-agen-properties`, `x-agen-type`, `x-agen-time-format`, `x-agen-validate` — deliberately mirroring ogen's `x-ogen-*` family for muscle memory.

---

## 8. Testing strategy (stolen wholesale from ogen)

- **Positive corpus**: `_testdata/positive/*` — generation must succeed and output must parse under `go/format` (`genfs.CheckFS`). Seed with the official AsyncAPI examples + every binding variant we claim.
- **Negative corpus**: `_testdata/negative/` — located errors, asserted including position.
- **Location tests**: error excerpts rendered from `_testdata/location/` specs.
- **Integration**: `internal/integration/` holds *committed* generated code regenerated via `go:generate` (per-case `agen.yml`); CI compiles + behavior-tests it (round-trips, ack/nack, middleware order, multi-message dispatch).
- **Broker integration**: testcontainers-go (Redis → Kafka → RabbitMQ/Mosquitto/NATS) gated by build tags, exercised against the committed integration packages.
- **Format coverage**: port ogen's `mkformattest` idea later — a generated spec exercising every format→type mapping.
- **Examples module**: `examples/` (own go.mod) regenerates real public AsyncAPI documents on CI to catch ecosystem drift.

---

## 9. Roadmap

| Milestone | Scope | Exit criteria |
|---|---|---|
| **M0 — skeleton** | Repo, Apache-2.0, CI (3 OS + race + lint), `cmd/agen` with config loading, spec loading, raw `asyncapi.Spec` DOM with locations, meta-schema validation, pretty errors | `agen` parses & validates the official 3.0 examples; negative corpus green |
| **M1 — models** | Semantic parser (channels/operations/messages/bindings, $ref, traits), vendored lowering core, emit **types + JSON codecs + validators only** (jschemagen parity). Includes the 1-day "synthetic OpenAPI through ogen" spike to validate the port | Streetlights payloads round-trip + validate; `_testdata/positive` green |
| **M2 — Redis end-to-end** | IR operations, publisher/handler/subscriber generation, middleware, fakes, `runtime/redis` (go-redis: Streams default + Pub/Sub opt-in, reclaimer + DLQ per §6.5), security config (ACL/TLS), address templating | testcontainers Redis integration suite green — ack / redelivery / DLQ paths covered |
| **M3 — Kafka** | `runtime/kafka` (franz-go) + binding fields per §6.3 (typed key, `groupId`/`clientId`, SASL/SSL) | testcontainers Kafka integration suite green |
| **M4 — more brokers** | MQTT (+v5), AMQP, NATS, WebSockets backends + binding fields per §6.3; header codecs vendored from `uri` | one committed integration package per protocol |
| **M5 — polish** | OTel instrumentation, request/reply (`operation.reply`), correlationId surfaces, Avro payload format, optional `--models-backend=modelina` (Node, opt-in), possible vacuum-based `agen lint` | — |

---

## 10. Risks and mitigations

| Risk | Mitigation |
|---|---|
| ogen's public `jsonschema` API churns | pin exact versions; single facade (`internal/schema`); ogen's release discipline is good (it's a library for its own CLI) |
| Copied lowering core drifts from upstream | `UPSTREAM.md` per vendored dir; keep file names identical; scheduled re-diff ritual; ogen's core schema files are mature/slow-moving |
| Dialect gaps (`not`, `$defs`, Avro payloads) surprise users | every gap is a *located, named* `ErrUnsupported` + `ignore_unsupported` escape hatch; documented support matrix |
| Multi-message dispatch ambiguity | ogen's sum-type machinery (discriminator / unique-field / type-based); undecidable shapes → generated ordered-try decode with explicit error; `x-agen-*` override knobs |
| Binding fragmentation (many stubs, version churn) | consume only fields we can act on; unknown version ⇒ warn + best-effort; stub bindings ⇒ address+host fallback |
| agen's Redis conventions diverge from a future official Redis binding | conventions confined to config + `x-agen-redis` extensions and isolated in `runtime/redis`; documented as agen-specific; adopting an official binding later is a codegen+runtime version bump, not a spec break for users |
| Name collisions across operation/channel/message IDs in one Go namespace | ogen naming + dedup machinery + `x-agen-name` override |
| Scope creep (protocols, registries) | milestones gated as above; unsupported-protocol errors carry the roadmap link |

---

## 11. Open questions

1. **Module hosting** — placeholder `github.com/agen-go/agen`; decide org vs personal namespace before M0 tags.
2. **Generated API ergonomics** — method-per-operation (`client.SendLightCommand`) vs channel-scoped publishers; envelope as one struct (`Headers`+`Payload`) vs separate args. Resolve with an M2 spike and a tiny usability doc before freezing templates.
3. **Redis default mode** — the design picks `streams` (durable, consumer groups, ack/redelivery) over `pubsub` as the default; confirm against the first real consumer app at M2 exit. **Kafka backend** (M3) — franz-go recommended (KIP coverage, throughput); sarama kept as a possible config-selectable alternative if enterprise oddities appear.
4. **Runtime module split** — one module for now (ogen-style); split `runtime/*` into its own low-churn module if generator iteration starts forcing user upgrades.

---

## Appendix: key references

- ogen: https://github.com/ogen-go/ogen · jschemagen as pipeline proof: `cmd/jschemagen`
- Modelina: https://github.com/asyncapi/modelina · docs: https://modelina.org · Go gaps: issues #2619, #2482
- AsyncAPI 3.0.0 spec: https://www.asyncapi.com/docs/reference/specification/3.0.0 · current: https://github.com/asyncapi/spec (`spec/asyncapi.md`)
- Bindings: https://github.com/asyncapi/bindings · docs: https://www.asyncapi.com/docs/reference/bindings
- Official meta-schemas: https://github.com/asyncapi/spec-json-schemas
- Parser contract reference: https://github.com/asyncapi/parser-api · behavioral oracle: https://github.com/asyncapi/parser-js
- Go prior art: https://github.com/lerenn/asyncapi-codegen · https://github.com/bdragon300/go-asyncapi · archived parser: https://github.com/asyncapi-archived-repos/parser-go
- Recommended runtime libs: redis/go-redis, twmb/franz-go, eclipse-paho/paho.mqtt.golang (+ paho.golang v5), rabbitmq/amqp091-go, nats-io/nats.go, coder/websocket
- Full research transcripts: `docs/research/{ogen,modelina,asyncapi-ecosystem}.md`
