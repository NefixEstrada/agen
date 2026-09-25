# AsyncAPI Ecosystem Research for "agen"

> Provenance: web research conducted 2026-09-23 (study phase). All version/status claims verified
> against primary sources unless noted. Source links inline.

---

## 1. AsyncAPI 3.0.x Specification Structure

### 1.1 Version status

- AsyncAPI 3.0.0 was released 2023-12-05; 3.0.1 (2023-12-13) contained no spec changes (tooling fix only). [Source: asyncapi/spec releases](https://github.com/asyncapi/spec/releases)
- The current spec in the repo is **3.1.0** (repo last commit 2026-09-13; the spec file is now at `spec/asyncapi.md` — the repo no longer uses `versions/*.md`). 3.1.0's headline change is adding ROS 2 bindings to the specification. [Source: asyncapi/spec](https://github.com/asyncapi/spec) / [releases](https://github.com/asyncapi/spec/releases)
- Official JSON Schemas exist for 1.0.0 → 3.1.0, including `-without-$id` variants (see §7). [Source: asyncapi/spec-json-schemas `schemas/` listing](https://github.com/asyncapi/spec-json-schemas/tree/master/schemas)

### 1.2 Root document object (3.0.x)

Fields: `asyncapi` (required, version string), `id` (URI/URN), `info` (required), `defaultContentType`, `servers` (map name → Server), `channels` (Channels Object), `operations` (Operations Object), `components`. Channels/operations are **optional at root** — a document with only `components` is valid in v3. [Source: AsyncAPI 3.0.0 spec](https://www.asyncapi.com/docs/reference/specification/3.0.0)

Key sub-objects a **parser must model**:

| Object | Fields (3.0.x) |
|---|---|
| **Info** | `title`, `version` (required), `description`, `termsOfService`, `contact`, `license`, `tags`, `externalDocs` (tags/externalDocs moved from root to Info in 3.0) |
| **Server** | `host` (required, supports `{variable}` templating), `protocol` (required), `pathname`, `protocolVersion`, `title`, `summary`, `description`, `variables` (map → Server Variable or `$ref`), `security` (array of SecurityScheme or `$ref`), `tags`, `externalDocs`, `bindings` |
| **Server Variable** | `enum` [string], `default` (string), `description`, `examples` [string] |
| **Channel** | `address` (string or **null** = unknown; supports `{param}` expressions), `messages` (**Messages Object**: map key → Message or `$ref`, keyed by messageId), `title`, `summary`, `description`, `servers` (**$ref-only array** — must point to root servers), `parameters`, `tags`, `externalDocs`, `bindings` |
| **Parameter** | Restricted in 3.0 to `enum`, `default`, `description`, `examples`, `location`; all parameters default to type string |
| **Operation** | `action` (**required**: `"send"` or `"receive"`), `channel` (**required $ref** to a channel), `title`, `summary`, `description`, `security`, `tags`, `externalDocs`, `bindings`, `traits`, `messages` (array of `$ref` — a **subset of the channel's messages**), `reply` |
| **Operation Reply** | `address` (Operation Reply Address or `$ref`), `channel` ($ref; must be null if `address` set), `messages` (array of `$ref`) |
| **Operation Reply Address** | `location` (**required**, runtime expression e.g. `$message.header#/replyTo`), `description` |
| **Operation Trait** | Any Operation field **except** `action`, `channel`, `messages`, `traits` |
| **Message** | `headers`, `payload`, `correlationId` (Correlation ID Object or `$ref`: `location` required + `description`), `contentType`, `name` (machine-friendly), `title`, `summary`, `description`, `tags`, `externalDocs`, `bindings`, `examples` (Message Example: `headers`, `payload`, `name`, `summary`), `traits` |
| **Message Trait** | Any Message field except `payload` and `traits` |
| **Multi Format Schema** | `schemaFormat` + `payload`/schema — replaces Message-level `schemaFormat` (removed in 3.0) |
| **Components** | Maps: `servers`, `serverVariables`, `channels`, `operations`, `messages`, `schemas`, `securitySchemes`, `parameters`, `correlationIds`, `replies`, `replyAddresses`, `operationTraits`, `messageTraits`, `serverBindings`, `channelBindings`, `operationBindings`, `messageBindings`, `tags`, `externalDocs`. Keys must match `^[a-zA-Z0-9.\-_]+$` |

[Sources: AsyncAPI 3.0.0 spec reference](https://www.asyncapi.com/docs/reference/specification/3.0.0), [3.1.0 spec source](https://github.com/asyncapi/spec/blob/master/spec/asyncapi.md)]

### 1.3 Semantic changes vs 2.6

- **Channels vs operations split.** In 2.x, channels were "directly coupled with operations" (channel items contained `publish`/`subscribe` operation objects). In 3.0, channels purely define addresses (topics/paths) plus their messages; operations live at root and reference channels via `$ref`. [Source: AsyncAPI 3.0.0 release notes](https://www.asyncapi.com/blog/release-notes-3.0.0)
- **send/receive from the APPLICATION's perspective.** The 3.x spec states verbatim: *"Use `send` when it's expected that the application will send a message to the given channel, and `receive` when the application should expect receiving messages from the given channel."* [Source: 3.1.0 spec, Operation Object](https://github.com/asyncapi/spec/blob/master/spec/asyncapi.md)]. The 2.6.0 spec's literal wording was the notorious inversion:
  - `subscribe` = "messages **produced by the application** and sent to the channel" (i.e., app sends)
  - `publish` = "messages **consumed by the application** from the channel" (i.e., app receives)
  [Source: 2.6.0 spec, Channel Item Object](https://github.com/asyncapi/spec/blob/v2.6.0/spec/asyncapi.md)]

  So the migration mapping is 2.x `subscribe` → 3.0 `send`, 2.x `publish` → 3.0 `receive` (a trap for anyone porting 2.x tooling — verify against your own assumptions). [Context: atamel.dev comparison](https://atamel.dev/post/asyncapi-gets-a-new-version-3-0/), [Nordic APIs overview](https://nordicapis.com/what-is-new-in-asyncapi-v3-0/)
- **Server object split**: v2's single `url` became `host` + `pathname` + `protocol` in v3; Server gained `title`, `summary`, `externalDocs`. Server variables remain `enum`/`default`/`description`/`examples`. [Source: 3.0.0 release notes](https://asyncapi.com/blog/release-notes-3.0.0)
- **Messages referenced from operations**: in 2.x, the message (`message` or `oneOf[]`) lived inside the channel's operation object; in 3.0 messages are defined on the **channel** (as a map keyed by messageId) and an operation's optional `messages` array is an array of `$ref`s selecting a subset. [Source: 3.0.0 release notes](https://www.asyncapi.com/blog/release-notes-3.0.0)
- **Request/reply**: new `reply` block on operations, with reply address discoverable at runtime (`$message.header#/replyTo`). [Source: 3.0.0 release notes](https://www.asyncapi.com/blog/release-notes-3.0.0)
- Other breaking changes: only explicit `$ref` references allowed (implicit name-based references removed); trait properties MUST NOT override the target object; `schemaFormat` moved from Message into the Multi Format Schema Object; parameters restricted; components extended. [Source: 3.0.0 release notes](https://www.asyncapi.com/blog/release-notes-3.0.0)

### 1.4 JSON Schema draft

- Both 2.6 and 3.x define the Schema Object as a **superset of JSON Schema Draft 07**. [Source: 2.6.0 spec](https://github.com/asyncapi/spec/blob/v2.6.0/spec/asyncapi.md) (line: "This object is a superset of the JSON Schema Specification Draft 07"), [3.1.0 spec](https://github.com/asyncapi/spec/blob/master/spec/asyncapi.md)]
- Default schema format if unspecified: `application/vnd.aai.asyncapi;version={{asyncapi-version}}` (e.g., `...;version=2.6.0` in 2.6). MUST-support formats: the AsyncAPI Schema Object itself and JSON Schema Draft 07 (`application/schema+json;version=draft-07`). RECOMMENDED: Avro 1.9.0, OpenAPI 3.0 Schema, RAML 1.0, Protobuf. [Sources: 2.6.0 spec schema format table](https://github.com/asyncapi/spec/blob/v2.6.0/spec/asyncapi.md), [3.0.0 spec](https://www.asyncapi.com/docs/reference/specification/3.0.0)]
- Practical takeaway for **agen**: a draft-07 JSON Schema engine covers the payload/headers vocabulary; `boolean` schemas (`true`/`false`) are legal everywhere.

---

## 2. `$ref` Resolution Rules and Schema Deviations

### 2.1 Reference resolution semantics

- The Reference Object "is defined by JSON Reference ([draft-pbryan-zyp-json-ref-03](https://tools.ietf.org/html/draft-pbryan-zyp-json-ref-03)) and follows the same structure, behavior and rules", and critically: **"For this specification, reference resolution is done as defined by the JSON Reference specification and not by the JSON Schema specification."** [Source: AsyncAPI spec, Reference Object](https://github.com/asyncapi/spec/blob/master/spec/asyncapi.md)]
- `$ref` may target: the same document, another local file, or an external URL. [Source: AsyncAPI docs, Reusable parts](https://www.asyncapi.com/docs/concepts/reusable-parts)
- **`$id` handling**: the spec defines no `$id`-based base-URI machinery. Base URI is inferred from the input source (file path/URL), which has caused real tooling inconsistencies; the community discussion proposes treating the nearest `$id` as base URI but this is not normative. The archived official Go parser listed "JSON Schema `$id` support" as an unrealized roadmap item. [Sources: asyncapi/event-gateway issue #782 / reference tooling discussion](https://github.com/asyncapi/event-gateway/issues/782), [archived parser-go README](https://github.com/asyncapi-archived-repos/parser-go)]
- Corollary for tooling: `spec-json-schemas` ships `-without-$id` variants of the official schemas precisely so `$id` doesn't break local reference resolution. [Source: spec-json-schemas](https://github.com/asyncapi/spec-json-schemas)
- In 3.0, several relationships are **$ref-only by construction**: `operation.channel` MUST be a Reference Object (MUST NOT embed a Channel Object) and root operations must point to root channels only; `channel.servers` must be `$ref`s to root servers. The spec RECOMMENDS parsers dereference `operation.channel` for ergonomics. [Source: 3.1.0 spec, Operation Object](https://github.com/asyncapi/spec/blob/master/spec/asyncapi.md)]
- In 2.6, `$ref` inside a Channel Item was already **deprecated** (overlapping fields → "behavior is undefined"). [Source: 2.6.0 spec, Channel Item Object](https://github.com/asyncapi/spec/blob/v2.6.0/spec/asyncapi.md)]
- Trait merging (3.0): trait properties **MUST NOT override** the same property on the target object. [Source: 3.0.0 release notes](https://www.asyncapi.com/blog/release-notes-3.0.0)

### 2.2 AsyncAPI Schema Object deviations from plain JSON Schema

All keywords of JSON Schema Core/Validation draft-07 work identically (`type`, `required`, `allOf/oneOf/anyOf/not`, `format`, etc.). Explicit deviations (per the spec's Schema Object section, [3.1.0 source](https://github.com/asyncapi/spec/blob/master/spec/asyncapi.md)):

1. **`$ref` behavior**: when a Schema Object contains `$ref`, it MUST follow the **Reference Object** (JSON Reference) behavior, not JSON Schema's `$ref`/`$id` resolution.
2. **`description`**: CommonMark allowed (editorial).
3. **`format`**: AsyncAPI adds extra predefined formats beyond JSON Schema's ([Data Type Formats section](https://github.com/asyncapi/spec/blob/master/spec/asyncapi.md)).
4. **`default`**: unlike JSON Schema, the value MUST conform to the sibling `type`.
5. **Added fields**: `discriminator` (string; property MUST be in `required`; powers polymorphism with `allOf` composition — inline schemas without an id cannot be used in polymorphism), `externalDocs`, `deprecated` (default false).
6. Boolean schemas (`true` = allow anything, `false` = allow nothing) are legal.
7. `additionalProperties`/`readOnly`/`writeOnly` etc. follow JSON Schema; the object "MAY be extended with Specification Extensions" (`x-*`) everywhere.

Note: `nullable` does **not** appear anywhere in either the 2.6.0 or 3.1.0 spec text (verified by grep) — despite common belief, it is not a spec-defined keyword.

---

## 3. Protocol Bindings Catalog

Canonical definitions live in the [asyncapi/bindings](https://github.com/asyncapi/bindings) repo (Apache-2.0, active — last commit 2026-07-30, 177 commits), one folder per protocol; browsable docs at [asyncapi.com/docs/reference/bindings](https://www.asyncapi.com/docs/reference/bindings). Bindings attach to servers, channels, operations, and messages, always with a `bindingVersion` field (omitted means "latest"). Fields extracted from the binding definitions in the repo (cloned and inspected directly).

| Protocol | bindingVersion | Server binding | Channel binding | Operation binding | Message binding | Notes / maturity |
|---|---|---|---|---|---|---|
| **kafka** | 0.5.0 | `schemaRegistryUrl`, `schemaRegistryVendor` | `topic`, `partitions`, `replicas`, `topicConfiguration` (cleanup.policy, retention.ms, retention.bytes, max.message.bytes, confluent key/value schema validation…) | `groupId`, `clientId` (Schema/Ref) | `key` (Schema/Ref/AVRO), `schemaIdLocation`, `schemaIdPayloadEncoding`, `schemaLookupStrategy` | Richest, most-used binding; schema-registry aware. [Docs](https://www.asyncapi.com/docs/reference/bindings/kafka) |
| **mqtt** | 0.2.0 | `clientId`, `cleanSession`, `lastWill` (topic/qos/message/retain), `keepAlive`; v5-only: `sessionExpiryInterval`, `maximumPacketSize` | (empty, reserved) | `qos`, `retain`; v5: `messageExpiryInterval` | v5-only: `payloadFormatIndicator`, `correlationData`, `contentType`, `responseTopic` | [Docs](https://www.asyncapi.com/docs/reference/bindings/mqtt) |
| **mqtt5** | 0.2.0 | `sessionExpiryInterval` only (rest reserved) | empty | empty | empty | Intentionally minimal. [Repo](https://github.com/asyncapi/bindings/tree/master/mqtt5) |
| **amqp** (0-9-1) | 0.3.0 | (empty) | `is`: `routingKey` (default) or `queue`; `exchange` {name, type, durable, autoDelete, vhost} or `queue` {name, durable, exclusive, autoDelete, vhost} | `expiration`, `userId`, `cc`, `priority`, `deliveryMode` (1/2), `mandatory`, `bcc`, `timestamp`, `ack` | `contentEncoding`, `messageType` | RabbitMQ world. [Docs](https://www.asyncapi.com/docs/reference/bindings/amqp) |
| **amqp1** (1.0) | 0.1.0 | all empty/reserved | empty | empty | empty | Definition is a stub. [Repo](https://github.com/asyncapi/bindings/tree/master/amqp1) |
| **nats** | 0.1.0 | empty | empty | `queue` (queue-group name) | empty | Very small surface. [Repo](https://github.com/asyncapi/bindings/tree/master/nats) |
| **websockets** (aka `ws`) | 0.1.0 | empty | `method`, `query`, `headers` | empty | `headers` | [Repo](https://github.com/asyncapi/bindings/tree/master/websockets) |
| **http** | 0.3.0 | empty | `method`, `query` | `method`, `query` | `statusCode`, `headers` | Mostly for webhooks/streaming. [Repo](https://github.com/asyncapi/bindings/tree/master/http) |
| **googlepubsub** | 0.2.0 | empty | `topic`, `schema`, `messageRetentionDuration`, `messageStoragePolicy` | `ackDeadline`? (orderingKey, attributes, schemaSettings in message/op) | `orderingKey`, `attributes`, `schemaSettings`… | [Repo](https://github.com/asyncapi/bindings/tree/master/googlepubsub) |
| **sns** | — | — | `topicName`, content-based dedup, policies | subscriptions (endpoints, filterPolicy, rawMessageDelivery, redrive) | — | Defined only as `2.x.x`/`3.0.0` subfolders, no top README "Current version". [Repo](https://github.com/asyncapi/bindings/tree/master/sns) |
| **sqs** | 0.3.0 | — | `queue` {name, fifoQueue, deduplicationScope…}, `deadLetterQueue`, `visibilityTimeout`, `maxReceiveCount`, policies | — | — | [Repo](https://github.com/asyncapi/bindings/tree/master/sqs) |
| **redis** | 0.1.0 | all objects empty/reserved | empty | empty | empty | Stub only. [Repo](https://github.com/asyncapi/bindings/tree/master/redis) |
| **pulsar** | 0.1.0 | — | `namespace`, `persistence`, `compaction`, `deduplication`, `retention` {time, size}, `ttl` | `tenant`? | — | Tenant/namespace in server/channel. [Repo](https://github.com/asyncapi/bindings/tree/master/pulsar) |
| **jms** | — | `clientId`? | `destination`, `destinationType` | — | `headers` | Contains v1-style markers. [Repo](https://github.com/asyncapi/bindings/tree/master/jms) |
| **solace** | 0.4.0 | — | `destinations`, `destinationType`, `queue` {accessType, maxMsgSpoolSize, maxTtl, topicSubscriptions} | `clientName`, `deliveryMode`, `dmqEligible`, `priority`, `timeToLive` | — | Vendor (Solace) maintained, quite rich. [Repo](https://github.com/asyncapi/bindings/tree/master/solace) |
| **ibmmq** | 0.1.0 | `queueManager`, `ccd…`, cipherSpec, multiEndpointServer… | `destinationType`, `queue`, `topic`, `maxMsgLength`… | `priority`, `persistence`… | `type`, `headers`, `expiry`, `groupId` | Vendor maintained. [Repo](https://github.com/asyncapi/bindings/tree/master/ibmmq) |
| **anypointmq** | — | `host`, `pathname`… | `destination`, `destinationType` | — | `headers` | [Repo](https://github.com/asyncapi/bindings/tree/master/anypointmq) |
| **mercure** | 0.1.0 | all empty/reserved | empty | empty | empty | Stub. [Repo](https://github.com/asyncapi/bindings/tree/master/mercure) |
| **stomp** | 0.1.0 | all empty/reserved | empty | empty | empty | Stub. [Repo](https://github.com/asyncapi/bindings/tree/master/stomp) |
| **ros2** | 0.1.0 | `domainId`, `rmwImplementation`… | `node`, `topic`… | `deadline`, `lifespan`, `liveliness`… | — | Newest; added in spec 3.1.0. [Repo](https://github.com/asyncapi/bindings/tree/master/ros2), [spec 3.1.0](https://github.com/asyncapi/spec/releases) |
| **zmq** | — | — | — | — | — | **Does not exist** in the official bindings repo or docs nav (verified by cloning the repo and the [docs index](https://www.asyncapi.com/docs/reference/bindings)); community proposal only |

Important observation for the generator design: many bindings are **empty stubs** (amqp1, redis, stomp, mercure, and most of mqtt5) — real-world usage for those protocols relies on channel `address` + server `host` alone.

**Maturity/popularity**: no public AsyncAPI survey with per-protocol percentages was found. Based on binding-definition richness and general ecosystem weight (Kafka dominant in event streaming, MQTT dominant in IoT, AMQP/RabbitMQ in enterprise messaging, NATS/WebSockets in web-native stacks), the sensible **MVP set is: Kafka, MQTT, AMQP (RabbitMQ), NATS, and WebSockets** — Kafka+MQTT+AMQP cover the overwhelming majority of real documents, NATS and WebSockets are cheap to add (both tiny binding surfaces). (Popularity ranking is an assessment, not a measured statistic.)

---

## 4. Existing AsyncAPI → Go Generators and Go Libraries

GitHub search (`asyncapi language:go`, sorted by stars, executed 2026-09-23) plus targeted follow-ups:

| Project | Stars | Status (last push) | Approach | AsyncAPI versions | Protocols | License | Weaknesses |
|---|---|---|---|---|---|---|---|
| **[lerenn/asyncapi-codegen](https://github.com/lerenn/asyncapi-codegen)** — the project usually meant by "asyncapi-go-code-generator" | 161 | Active (2026-06-14), v0.63.0 | Go CLI, `go install`-able; generates one `.go` file (app + user boilerplate + types) | **2.6.0 and 3.0.0** | **Kafka** (SASL/TLS, group/partitions, commit), **NATS + JetStream**; pluggable `BrokerController` interface for custom brokers; **no MQTT/AMQP** | Apache-2.0 | Single-file output; partial spec coverage (author admits "some features may still be missing"); validation gaps (`required` needs `--force-pointers`, string-only enums); JetStream ack caveat; ambiguous multi-message discrimination; runtime dep on its own `pkg/extensions` + `go-playground/validator` |
| **[bdragon300/go-asyncapi](https://github.com/bdragon300/go-asyncapi)** | 12 | Active dev, self-described **unstable** (2026-08-22) | Go CLI (no Node/Docker); template-driven codegen extensible "only by writing Go templates"; also diagram/docs/flatten/validate CLI clients | Claims full spec (unversioned) | Abstract core + concrete: AMQP (amqp091-go), HTTP (net/http), Kafka (franz-go), MQTT v3 (paho.mqtt.golang), **MQTT v5 (eclipse/paho.golang)**, NATS (nats.go), Redis (go-redis), TCP/UDP/IP, WebSockets (gobwas/ws) | Apache-2.0 | Small community, no releases, "API may change"; template-based = weaker typing guarantees than AST codegen |
| **[asyncapi-archived-repos/parser-go](https://github.com/asyncapi-archived-repos/parser-go)** — official Go parser | 58 | **Archived 2025-08-04**, read-only | Pure Go parser/validator package + CLI; validates against JSON Schemas, dereferences, outputs JSON | 2.x only (1.x dropped); no 3.x | n/a (parser) | Apache-2.0 | Archived with **no successor**; roadmap items (Avro, `$id`, extensions) never done; built by Kyma devs + AsyncAPI founders |
| **[swaggest/go-asyncapi](https://github.com/swaggest/go-asyncapi)** | 90 | Maintained (2025-11-20) | **Spec-from-code** (reverse direction): builds AsyncAPI documents from Go structs; not a client generator | 2.6 | n/a | MIT | Not a codegen target for agen; useful prior art for DOM typing |
| **[daveshanley/vacuum](https://github.com/daveshanley/vacuum)** | 1128 | Very active (2026-09-15) | Linter/docs toolkit for OpenAPI **and AsyncAPI** (Spectral-compatible rulesets) | AsyncAPI 2.x (3.x partial) | n/a | MIT | Linting only, not codegen — but a good validation dependency/reference |
| **[marle3003/mokapi](https://github.com/marle3003/mokapi)** | 58 | Active (2026-09-22) | API mocking (OpenAPI + AsyncAPI) in Go/JS | 2.x/3.x | Kafka, AMQP, MQTT, NATS mocked | (see repo) | Mocking, not codegen |
| **[polanski13/asyngo](https://github.com/polanski13/asyngo)** | 10 | Active (2026-07-24) | Annotations → AsyncAPI 3.1 spec ("swaggo for event-driven") — spec-from-code | 3.1 | WebSocket etc. | none listed | Reverse direction |
| **[asyncapi-archived-repos/go-watermill-template](https://github.com/asyncapi-archived-repos/go-watermill-template)** | 60 | **Archived** | Official generator template producing Go code via the [ThreeDotsLabs/watermill](https://github.com/ThreeDotsLabs/watermill) message-routing library | 2.x-era | AMQP (Watermill) | Apache-2.0 | Archived, unmaintained; **no actively maintained Go template exists for the official generator today** ([generator README](https://github.com/asyncapi/generator/blob/master/README.md)) |
| **[jposton96a/to-go](https://github.com/jposton96a/to-go)** | 1 | Dead (2020-09-14) | Template for the official asyncapi/generator | old 2.x | AMQP | — | Abandoned |
| **[dimonoff/asyncapi-codegen](https://github.com/dimonoff/asyncapi-codegen)** | 1 | Active fork of lerenn (2026-05-11) | Fork with company-specific extensions | as upstream | as upstream | Apache-2.0 | Fork drift |
| **[ktartsch/async-to-go](https://github.com/ktartsch/async-to-go)** / **[plheide/aapi-codegen](https://github.com/plheide/aapi-codegen)** | 1 each | 2026 | Small personal generators (typed payloads; AMQP publishers; "counterpart to oapi-codegen") | 3.x | AMQP | — | Hobby scale |
| "gogen-asyncapi" | — | — | **No such repository exists** on GitHub (verified via search API and web search); closest matches are the above | | | | |

Additional facts:

- The **official asyncapi/generator** is a Node.js/Turborepo monorepo (1,076 stars, Apache-2.0, pushed 2026-09-21) using Nunjucks/React templates; its supported templates today are html, markdown, java JMS, java-spring-cloud-stream — **all previous Go/Node/Python/PHP/.NET code templates were archived**, including `@asyncapi/go-watermill-template`. [Source: generator README](https://github.com/asyncapi/generator/blob/master/README.md)
- The archived official **converter-go** ([asyncapi-archived-repos/converter-go](https://github.com/asyncapi-archived-repos/converter-go), 20 stars, archived) converted older AsyncAPI versions in Go — also dead.
- Gap analysis for agen: nothing in the Go ecosystem today offers (a) 3.x-first support, (b) ogen-style typed codegen with a clean runtime package, and (c) >2 protocol backends simultaneously — lerenn has Kafka+NATS only; bdragon300 is closest in ambition but explicitly unstable with no releases.

---

## 5. Recommended Go Client Libraries for the Runtime Package

Status checked via GitHub API on 2026-09-23 (stars / last push / license):

| Protocol | Options | Recommendation |
|---|---|---|
| **Kafka** | [twmb/franz-go](https://github.com/twmb/franz-go) (3,069★, pushed 2026-09-18, BSD-3) · [IBM/sarama](https://github.com/IBM/sarama) (12,521★, 2026-09-22, MIT) · [segmentio/kafka-go](https://github.com/segmentio/kafka-go) (8,630★, 2026-04-23, MIT) | **franz-go** — modern KIP-compliant client with the best support for newer Kafka features, high throughput, and an explicit goal of full protocol coverage; sarama is battle-tested and unavoidable for odd enterprise needs (keep as alternative backend); kafka-go has the friendliest API but lags on newest features |
| **MQTT 3.1.1** | [eclipse-paho/paho.mqtt.golang](https://github.com/eclipse-paho/paho.mqtt.golang) (3,124★, 2026-09-23; note the repo moved from `eclipse/` to `eclipse-paho/`) | **paho.mqtt.golang** — the de-facto standard, actively maintained |
| **MQTT 5** | [eclipse/paho.golang](https://github.com/eclipse/paho.golang) | **paho.golang** for v5-first designs (this is what bdragon300/go-asyncapi uses for MQTT5); expose v3 via paho.mqtt.golang if you need both |
| **AMQP 0-9-1 (RabbitMQ)** | [rabbitmq/amqp091-go](https://github.com/rabbitmq/amqp091-go) (2,038★, 2026-09-23) | **amqp091-go** — the only real option, officially maintained by the RabbitMQ team (successor of streadway/amqp) |
| **NATS / JetStream** | [nats-io/nats.go](https://github.com/nats-io/nats.go) (6,759★, 2026-09-18, Apache-2.0) | **nats.go** — official client, first-class JetStream; no contest |
| **WebSockets** | [gorilla/websocket](https://github.com/gorilla/websocket) (24,869★, last push 2025-03-19, BSD-2) · [coder/websocket](https://github.com/coder/websocket) (5,477★, 2026-06-15, ISC; formerly nhooyr.io/websocket) | **coder/websocket** for new code — context-first API, `net/http`-compatible, zero deps, actively pushed; **gorilla/websocket** if maximum ecosystem familiarity matters |
| **Redis (pub/sub)** | [redis/go-redis](https://github.com/redis/go-redis) (22,247★, 2026-09-21, BSD-2) | **go-redis** — official Redis client, pub/sub included |
| **Google Pub/Sub** | [googleapis/google-cloud-go](https://github.com/googleapis/google-cloud-go) `cloud.google.com/go/pubsub` (4,505★, 2026-09-22) | **cloud.google.com/go/pubsub** — only sanctioned option |
| **Pulsar** | [apache/pulsar-client-go](https://github.com/apache/pulsar-client-go) (745★, 2026-09-23, Apache-2.0) | **pulsar-client-go** — official Apache client; works but smallest community of the set |

---

## 6. ogen (ogen-go/ogen) and AsyncAPI

- **Zero issues or PRs** in ogen-go/ogen mention AsyncAPI, Kafka, or event-driven APIs (GitHub issue/PR search API: `repo:ogen-go/ogen asyncapi` → `total_count: 0`; same for `kafka` and `event-driven`). [Search link](https://github.com/search?q=repo%3Aogen-go%2Fogen+asyncapi&type=issues)
- ogen **has Discussions enabled** (~25 open across 3 pages); none mention AsyncAPI/Kafka/event-driven. The nearest is "Supporting Server-Sent Events" (SSE over HTTP), plus "Roadmap/where is this project headed?" — no AsyncAPI plans stated there. [Source: ogen discussions](https://github.com/ogen-go/ogen/discussions)
- ogen positions itself strictly as an **OpenAPI v3 code generator**; its only streaming-adjacent feature is `text/event-stream` support. [Source: ogen repo / pkg.go.dev](https://github.com/ogen-go/ogen)
- Conclusion: no overlap, prior art, or stated intent in ogen for event-driven APIs — "agen" would not collide with an announced ogen roadmap item.

---

## 7. Official AsyncAPI Tooling Landscape Relevant to a Generator

- **Official JSON Schemas** — [asyncapi/spec-json-schemas](https://github.com/asyncapi/spec-json-schemas) (75★, Apache-2.0, pushed 2026-09-22): `schemas/{1.0.0 … 2.6.0, 3.0.0, 3.1.0}.json` plus `-without-$id` variants and `all.schema-store.json` (for editors). This is what a Go validator should embed or fetch. [Schemas listing](https://github.com/asyncapi/spec-json-schemas/tree/master/schemas)
- **asyncapi/generator** ([repo](https://github.com/asyncapi/generator), 1,076★): the official template engine (Node.js, Nunjucks + React rendering, hooks system, Turborepo monorepo). Go's official template (`go-watermill-template`) is archived — the maintained template set is html, markdown, java-template, java-spring-cloud-stream. [README](https://github.com/asyncapi/generator/blob/master/README.md)
- **asyncapi/studio** ([repo](https://github.com/asyncapi/studio), 213★, active): the web-based visual editor at studio.asyncapi.com — emits standard documents, nothing exotic. [Repo](https://github.com/asyncapi/studio)
- **parser-api** ([repo](https://github.com/asyncapi/parser-api), 11★, Apache-2.0): the "Global API definition for all AsyncAPI Parser implementations" — an intent-driven, spec-version-independent DOM contract (AsyncAPIDocument, Channel, Operation, OperationReply, Message, Server, Binding, Schema "superset of JSON Schema Draft 07", etc.). Any new parser (including a Go one) is invited to follow it; parser-go is cited as the conformance example. [README](https://github.com/asyncapi/parser-api)
- **parser-js** ([repo](https://github.com/asyncapi/parser-js), 144★, pushed 2026-09-09): the reference JS parser (browser-compatible) used by generator/studio — the de-facto behavioral oracle for $ref/dereference/validation semantics. [Repo](https://github.com/asyncapi/parser-js)
- **Modelina** ([repo](https://github.com/asyncapi/modelina), 448★, Apache-2.0, pushed 2026-09-13): generates data models from AsyncAPI 2.0–3.0 (plus OpenAPI/Swagger, JSON Schema d4-d7, Avro, RAML, XSD) into Go, Java, TypeScript, C#, Rust, Python, and more; Node-only. Major version roughly every 3 months; only latest major maintained. See `docs/research/modelina.md`. [README](https://github.com/asyncapi/modelina)
- **vacuum** ([repo](https://github.com/daveshanley/vacuum), 1,128★): fastest Spectral-compatible linter for OpenAPI/AsyncAPI in Go — candidate dependency for agen's `lint`/`validate` subcommands. [Repo](https://github.com/daveshanley/vacuum)
- **Converter**: official version conversion is JS-only (`@asyncapi/converter`); the Go converter ([asyncapi-archived-repos/converter-go](https://github.com/asyncapi-archived-repos/converter-go)) is archived — if agen wants 2.x input support it must ship its own 2.x→3.x transform.

---

## Key Takeaways for the agen Design Document

1. **Target 3.0.x (+3.1.0) first**; 2.6 input later via an internal transform. The DOM to parse is: root(info, servers{host,pathname,protocol,variables,security}, channels{address,messages,parameters,bindings}, operations{action,channel$ref,messages$ref[],reply,bindings}, components{17 maps}).
2. **$ref discipline**: plain JSON Reference semantics (not JSON Schema `$id` chains); internal, local-file, and remote-URL refs; `operation.channel`/`channel.servers` as ref-only links; traits must-not-override (3.0 rule).
3. **Schemas**: a draft-07 superset engine suffices; add `discriminator`, `externalDocs`, `deprecated`, type-conforming `default`, boolean schemas, and per-object `schemaFormat` dispatch (Avro/OpenAPI/RAML as opt-ins).
4. **MVP protocols**: Kafka, MQTT, AMQP, NATS, WebSockets — mapping to franz-go, paho (v3/v5), amqp091-go, nats.go, coder/websocket. Most other official bindings are empty stubs, so their generators collapse to "address + host" anyway.
5. **Competitive gap is real**: the only serious Go generator (lerenn/asyncapi-codegen) covers Kafka+NATS with partial spec support; the official Go parser and Go generator template are both archived; bdragon300/go-asyncapi is the closest full-stack effort but explicitly unstable with zero releases. No ogen AsyncAPI plans exist.
