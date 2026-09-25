# AsyncAPI Modelina — Research Report for "agen"

> Provenance: research conducted 2026-09-23 (study phase). Sources: local shallow clone of
> https://github.com/asyncapi/modelina at `/tmp/agen-study/modelina` (master @ `4dae829`, 2026-08-14,
> package version **5.10.1**), the docs site at **https://modelina.org/docs** (the old
> `modelina.asyncapi.io` domain no longer resolves), the GitHub API, and the npm registry.
> All Go-output claims were **verified by actually running** Modelina 5.10.1 (library via ts-node and
> the compiled CLI) against AsyncAPI 2.6.0 and 3.0.0 documents written for this study.

---

## 1. What Modelina is; supported inputs and input detection

**Modelina** (`@asyncapi/modelina`, Apache-2.0, Node ≥ 18) is the AsyncAPI Initiative's official *data-model* generation library: it converts API-specification documents into typed data models (classes/structs/enums) for 12 languages. It deliberately generates **only data models** — no client/server code, no operations, no transport logic. Its architecture is: *Input Processor* (raw doc → internal `InputMetaModel`) → *Interpreter* (JSON Schema → `MetaModel`) → *Constrainer* (language-specific names/types → `ConstrainedMetaModel`) → *Renderer* (presets → code strings).

### Supported inputs (registry in `src/processors/InputProcessor.ts`)

| Input | Details | Processor file |
|---|---|---|
| **AsyncAPI** | Versions **2.0.0 → 3.0.0** (see `supportedVersions` at `src/processors/AsyncAPIInputProcessor.ts:31-40`). AsyncAPI 3.1 not yet supported (open issue "AsyncAPI 3.1 support?"). Message payloads in schemaFormats: AsyncAPI Schema, JSON Schema draft-7, **Avro 1.9**, **RAML 1.0 datatype**, **OpenAPI 3.0 schema** (via `NewParser(3, { includeSchemaParsers: true })`, lines 72-75) | `src/processors/AsyncAPIInputProcessor.ts` |
| **OpenAPI 3.0/3.1 & Swagger 2.0** | Models generated from request/response payloads | `src/processors/OpenAPIInputProcessor.ts`, `src/processors/SwaggerInputProcessor.ts` |
| **Raw JSON Schema** | Draft-4, Draft-6, Draft-7 (default) | `src/processors/JsonSchemaInputProcessor.ts` |
| **Avro** | v1.x, requires `name` + `type` | `src/processors/AvroSchemaInputProcessor.ts` |
| **XSD** | via `fast-xml-parser` | `src/processors/XsdInputProcessor.ts` |
| **TypeScript types** | via `typescript-json-schema` | `src/processors/TypeScriptInputProcessor.ts` |
| **Meta model** | Hand-built internal `InputMetaModel` | (documented in `docs/usage.md`) |

**TypeSchema is NOT supported** (not in the processor registry). Inputs can be: JS objects, YAML/JSON strings, `file://` URIs, or pre-parsed documents from `@asyncapi/parser` (v1 old-parser objects also accepted).

### How detection works (`src/processors/InputProcessor.ts:52-66`)

An ordered map of processors is probed: `asyncapi` → `swagger` → `openapi` → `typescript` → `avro` → `xsd`; the **first whose `shouldProcess(input)` returns true wins**, otherwise the `default` (JSON Schema draft-7) processor is used. AsyncAPI detection (`AsyncAPIInputProcessor.shouldProcess`, lines 387-399) reads `input.asyncapi` (parsing YAML/JSON strings first) and checks membership in `supportedVersions`; `file://` string inputs are assumed AsyncAPI files. Practical implication: any document without an `asyncapi`/`openapi`/`swagger` field is treated as raw JSON Schema.

---

## 2. Output languages, focusing on Go

12 outputs: TypeScript, JavaScript, Java, Kotlin, Python, Rust, C#, C++, PHP, Dart, Scala, **Go**. Per the README features table, Go has the **thinnest feature set of all languages**: *"Go — Struct and enum generation: custom indentation type and size, etc"* (compare TypeScript: un/marshal functions, examples; C#: serializers; Rust: serde macros).

### Go generator source layout (`src/generators/go/`)

- `src/generators/go/GoGenerator.ts` — main class; options incl. `packageName` (default `'AsyncapiModels'`, line 102), `unionAnyModelName`/`unionDictModelName`/`unionArrModelName` (lines 96-98); renders structs/enums/unions, warns and skips other model kinds (lines 190-192)
- `src/generators/go/GoFileGenerator.ts` — `generateToFiles()` writes one file per model, **snake_case filenames** (`light_measured.go`, line 31)
- `src/generators/go/renderers/StructRenderer.ts` — struct + `IsXTypeY()` discriminator marker methods for polymorphism
- `src/generators/go/renderers/EnumRenderer.ts` — enums as `uint` + `iota` + `Value() any` + lookup maps
- `src/generators/go/renderers/UnionRenderer.ts` — unions as struct of anonymous embedded fields (e.g. `type Union struct { string; float64; ModelinaAnyType interface{} }`) or as an `interface` with discriminator markers
- `src/generators/go/GoConstrainer.ts` — type mapping: `interface{}`, `float64`, `int`, `string`, `bool`, `[]T`, `map[K]V`; nullable/optional scalars → `*T` pointers (lines 24-47); **tuples → `[]interface{}`** (line 58); **no `time.Time`** — `format: date-time` stays `string` (open feature request [#2619](https://github.com/asyncapi/modelina/issues/2619))
- `src/generators/go/constrainer/{ModelNameConstrainer,PropertyKeyConstrainer,EnumConstrainer,ConstantConstrainer}.ts` — PascalCase formatting, Go reserved-keyword escaping (`src/generators/go/Constants.ts`), de-duplication
- `src/generators/go/presets/CommonPreset.ts` and `DescriptionPreset.ts` — see §3

### What generated Go code contains

- **Structs with fields only.** No constructors, no getters/setters, **no `MarshalJSON`/`UnmarshalJSON` methods for structs** — struct serialization is purely stdlib `encoding/json` via tags.
- **`json` struct tags** (opt-in via `GO_COMMON_PRESET` + `addJsonTag: true`): optional fields get `json:"name,omitempty"`; **required fields get `json:"name" binding:"required"` — a Gin-framework `binding` tag is unconditionally baked in** (`src/generators/go/presets/CommonPreset.ts:25`), an opinionated choice you cannot configure per-tag-kind from the CLI.
- **Every object gets an extra `AdditionalProperties map[string]interface{}` field** tagged `json:"-"`.
- **References to other models become pointers** (`*Members`), including required enum fields.
- **Enums**: `type X uint`, `iota` constants, `Value() any`, `XValues []any`, `ValuesToX map[any]X`, and (with the JSON-tag preset) `MarshalJSON`/`UnmarshalJSON` that go through `any` — functional but allocation-heavy and un-idiomatic (no `type Status string`).
- **No validation code whatsoever** (no min/max/pattern enforcement beyond the Gin `binding:"required"` tag). Modelina's philosophy: *"does not hardcode … validation annotations"* (`docs/other-tools.md`).
- **Output is not gofmt'ed** — 2-space indent by default (configurable type/size), tabs mixed in from the enum renderer; a consumer must run `gofmt`/`goimports`.

### Verified realistic Go output (AsyncAPI 2.6.0, `--packageName streetlights --goIncludeTags --goIncludeComments`)

Input: channel `light/measured` with `$ref: '#/components/schemas/lightMeasured'`; schema has `id` (required string), `lumens` (required int, min 0), `status` (enum on/off), `sentAt` (date-time).

```go
package streetlights

type LightMeasured struct {
	Id string `json:"id" binding:"required"`
	Lumens int `json:"lumens" binding:"required"`
	Status *AnonymousSchema_3 `json:"status,omitempty"`
	AdditionalProperties map[string]interface{} `json:"-,omitempty"`
}
```

```go
package streetlights
import (
  "encoding/json"
)
type AnonymousSchema_3 uint

const (
	AnonymousSchema_3On AnonymousSchema_3 = iota
	AnonymousSchema_3Off
)

// Value returns the value of the enum.
func (op AnonymousSchema_3) Value() any {
	if op >= AnonymousSchema_3(len(AnonymousSchema_3Values)) {
		return nil
	}
	return AnonymousSchema_3Values[op]
}

var AnonymousSchema_3Values = []any{"on","off"}
var ValuesToAnonymousSchema_3 = map[any]AnonymousSchema_3{
	AnonymousSchema_3Values[AnonymousSchema_3On]: AnonymousSchema_3On,
	AnonymousSchema_3Values[AnonymousSchema_3Off]: AnonymousSchema_3Off,
}

func (op *AnonymousSchema_3) UnmarshalJSON(raw []byte) error {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	*op = ValuesToAnonymousSchema_3[v]
	return nil
}

func (op AnonymousSchema_3) MarshalJSON() ([]byte, error) {
	return json.Marshal(op.Value())
}
```

Maturity/status: the Go generator **works and is snapshot-tested** (`test/generators/go/__snapshots__/`), but is a second-class citizen next to TypeScript/C#/Java/Rust — no marshalling preset, no XML/binary presets (`docs/languages/Go.md`: *"To and from XML: Currently not supported… To and from binary: Currently not supported"*), no `time.Time`, no pointer control, open Go feature requests (#2619).

---

## 3. Usage modes

### Node library (primary mode)

From `examples/generate-go-models/index.ts` and `examples/go-json-tags/index.ts`:

```ts
import { GoGenerator, GoFileGenerator, GO_COMMON_PRESET, GO_DESCRIPTION_PRESET } from '@asyncapi/modelina';

// Minimal
const generator = new GoGenerator();
const models = await generator.generate(asyncapiDocument); // or JSON Schema / OpenAPI object
for (const model of models) console.log(model.modelName, model.result);

// Recommended for Go
const generator2 = new GoFileGenerator({
  presets: [
    { preset: GO_DESCRIPTION_PRESET },
    { preset: GO_COMMON_PRESET, options: { addJsonTag: true } }
  ]
});
const models2 = await generator2.generateCompleteModels(input, { packageName: 'streetlights' });
await generator2.generateToFiles(input, './out', { packageName: 'streetlights' });
```

Each returned `OutputModel` has `result`, `modelName`, `model`, `dependencies`, `inputModel` (`docs/usage.md`). Three tiers of options: constructor (generator), render, and processor options.

### CLI (`@asyncapi/modelina-cli`, binary `modelina`; oclif-based; `modelina-cli/` in-repo)

```sh
npm install -g @asyncapi/modelina-cli   # or npx
modelina generate golang ./asyncapi.yaml --packageName streetlights \
  --goIncludeTags --goIncludeComments -o ./models
# LANGUAGE ∈ {typescript, csharp, golang, java, javascript, dart, python, rust, kotlin, php, cplusplus, scala}
# FILE = path, URL, or AsyncAPI "context" name; omit -o to print to stdout
```

`modelina-cli/src/helpers/go.ts` shows the CLI's **entire Go surface**: it requires `--packageName`, and the only Go flags are `--goIncludeComments` (adds `GO_DESCRIPTION_PRESET`) and `--goIncludeTags` (adds `GO_COMMON_PRESET` with `addJsonTag: true`). **The CLI cannot pass custom presets, constraints, type mappings, or indentation options** — those exist only in the library API.

### Presets relevant to Go (the complete list is two)

1. `GO_COMMON_PRESET` (`src/generators/go/presets/CommonPreset.ts`) — options `{ addJsonTag: boolean }`: JSON struct tags + enum `MarshalJSON`/`UnmarshalJSON`. This is Go's *only* serialization story.
2. `GO_DESCRIPTION_PRESET` (`src/generators/go/presets/DescriptionPreset.ts`) — renders `description` (and example) fields as Go comments on structs/fields/enums.

(For contrast, TypeScript also has `TS_COMMON_PRESET` with `marshalling: true` generating real `marshal/unmarshal` functions, JsonBinPack support, etc. — none of that exists for Go.)

### Hooks / customization points (library API only)

- **Presets** (`docs/presets.md`): layerable hooks — Go's hook surface (`src/generators/go/GoPreset.ts`): `struct.self|field|additionalContent|discriminator`, `enum.self|item|additionalContent`, `union.self|field|additionalContent`. Presets can prepend/append/replace rendered fragments and add dependencies via `renderer.dependencyManager.addDependency('encoding/json')`.
- **Constraints** (`docs/constraints/Go.md`): overridable rule sets for `modelName`, `propertyKey`, `enumKey`, `enumValue`, `constant` — e.g. `new GoGenerator({ constraints: { modelName: ({modelName}) => ... } })`. Default Go rules: special chars → name words, `number_` prefix, reserved keywords list, PascalCase formatter, `reserved_` de-dup.
- **Type mapping**: fully overridable `typeMapping` per meta-model kind (`examples/change-type-mapping`).
- **Indentation**: `indentation: { type: IndentationTypes.SPACES|TABS, size }` (`src/generator/AbstractGenerator.ts:40-45`).
- **Processor hooks**: `processorOptions` (e.g., `interpretSingleEnumAsConst`, `propertyNameForAdditionalProperties`, AsyncAPI parser options) and the ability to register custom input processors via `InputProcessor.processor.setProcessor(...)` (`src/processors/InputProcessor.ts:34`).
- **Logging**: pluggable `Logger.setLogger(...)` with debug/info/warn/error (`docs/advanced.md`).

---

## 4. How models are extracted from an AsyncAPI document

All logic in `src/processors/AsyncAPIInputProcessor.ts:process()` (lines 48-184):

- **Trigger: message payloads only.** It iterates `doc.channels()` → `channel.operations()` → `operation.messages()` (+ reply messages), and converts each message's `payload()` to a model. **Message headers are NOT processed** — verified empirically (a `headers` schema produced no model), confirmed by open feature request [#2482](https://github.com/asyncapi/modelina/issues/2482) (PRs #2484 and #2520 were closed *unmerged*). Significant gap for headers-carrying protocols like Kafka/HTTP.
- **Multiple messages per operation → treated as `oneOf`**: each payload becomes its own model, **plus** a synthetic channel-level union model whose `$id` is the channel id (`channel.id()`, lines 143-149). Verified: channel `light/command` with two `$ref`'d messages generated `type LightSlashCommand struct { CommandPayload \`json:"-"\` }` (channel address sanitized to PascalCase).
- **Fallback**: if there are no channels, `doc.allMessages()` is used (lines 174-181).
- **Name derivation** (`src/interpreter/Utils.ts:56`): `schema.title || schema.$id || schema['x-modelgen-inferred-name']`. For AsyncAPI, the parser-assigned schema id is stamped into `x-modelgen-inferred-name` during `convertToInternalSchema()` (line 221). Consequences, all verified by running:
  - `$ref` into `components/schemas/lightMeasuredPayload` → model `LightMeasuredPayload` (component key wins — good).
  - **Inline payload under a *named* message still yields `AnonymousSchema_1`** — `message.name`/`title` do **not** propagate to the model name. Anonymous sub-schemas likewise get `AnonymousSchema_2`, `AnonymousSchema_3`, … (numbering is document-order-dependent).
  - Channel-derived union names come from the channel address (`light/command` → `LightSlashCommand`).
  - The final Go identifier then passes through the Go model-name constrainer (PascalCase, reserved words, etc.).
- **`$ref` behavior**: the AsyncAPI parser dereferences; shared referenced schemas become separate models rendered once and referenced elsewhere as pointer fields. Within raw JSON Schema input, `$ref`s are resolved by `@apidevtools/json-schema-ref-parser` (`dereferenceInputs`), and `reflectSchemaNames()` walks the schema pre-dereference to infer names for anonymous subschemas (`allOf/0`, `propName`, indices → dedup with `_1` suffixes) and stamps `x-modelgen-inferred-name` (`src/processors/JsonSchemaInputProcessor.ts:271-319`).
- **Polymorphism limitation**: `discriminator`-based polymorphism does not produce Go inheritance; schemas are **merged** and a marker method `func (r Dog) IsPetType() {}` is generated so union types satisfy a parent interface (`docs/usage.md` "Limitations and Compatibility — Polymorphism", issue [#108](https://github.com/asyncapi/modelina/issues/108); renderer in `src/generators/go/renderers/StructRenderer.ts:76-97`).

---

## 5. Integration story for a Go program

**Modelina cannot be used from Go natively.** It is a TypeScript/Node library (plus a browser server-side build) with no cgo bindings, no gRPC/REST service, no official WASM artifact. A Go program has exactly two realistic options:

1. **Shell out to the Node CLI** (`modelina generate golang ... -o dir` or `npx @asyncapi/modelina-cli`). Cost: a **Node ≥ 18 runtime becomes a hard dependency of agen's toolchain** — unacceptable as a required dependency for a self-contained Go generator (`go install`/single-binary story dies), tolerable at best as an optional "backend" for users who already have Node. The CLI surface is also the *least* powerful (no custom presets/constraints/type-mapping — only `--packageName`, `--goIncludeTags`, `--goIncludeComments`, `-o`).
2. **Ship a small Node sidecar script** that imports `@asyncapi/modelina` as a library, reads a JSON request (doc + options incl. custom presets), and emits generated code / `OutputModel` metadata as JSON. More powerful than the CLI (full hook access, access to the constrained meta-model, could even emit the `InputMetaModel` IR for post-processing), but still requires Node at generation time.

A speculative third path — running Modelina's ESM bundle inside an embedded JS runtime in Go (e.g., goja) — is undocumented, untested, and blocked in practice by Node-ish dependencies in the dependency chain (`fs`, `path`, parser libs); do not plan around it. **Any Modelina reuse couples agen to a Node runtime for at least the model-generation step**, which conflicts with the typical "pure Go, `go install`-able" expectation of Go codegen tools (cf. ogen, protoc-gen-go being Go binaries).

---

## 6. License, maintenance, ecosystem relation

- **License:** Apache-2.0 (`/LICENSE`, copyright "2016-2025 AsyncAPI Initiative"; `NOTICE` present). Under the AsyncAPI Initiative / Linux Foundation umbrella.
- **Activity (as of 2026-09-23):** healthy. 448 stars, 59 open issues, last push 2026-09-13; latest stable **v5.10.1** on npm; **v6.0.0-next.18** prerelease published 2026-09-01 (v6 in active development). The README documents a deliberate ~3-month major-version cadence with strict "any generated-output change is breaking" semantics — expect frequent majors and pin versions. Maintenance policy: only the latest major is maintained.
- **Relation to asyncapi/generator:** Modelina **is** the official model-generating engine of the AsyncAPI ecosystem. The npm package `@asyncapi/generator-model-sdk` — historically used by the official AsyncAPI Generator templates — is marked deprecated with *"It is @asyncapi/modelina now"*, and the old GitHub repo redirects to `asyncapi/modelina`. The CLI also integrates AsyncAPI Studio "contexts". Official, but scoped to *models only* — the official generator's templates remain Nunjucks-based for everything else (clients, servers, docs).

---

## 7. Verdict for agen

### What Modelina could cover

- Payload-schema → Go `struct`/`enum` rendering for AsyncAPI 2.0-3.0 with multi-schemaFormat support (JSON Schema, Avro, RAML, OpenAPI-schema payloads) handled for free.
- A battle-tested naming-constraint pipeline (reserved words, PascalCase, de-dup) and the split/constrain meta-model architecture — excellent **design inspiration** even without reusing the code.
- Channel-level `oneOf` union synthesis (a useful pattern to replicate deliberately).
- Multi-language ambitions for free later (Java/TS models from the same AsyncAPI doc) — relevant if agen ever wants polyglot output.

### What is inherently missing for agen's goals (Modelina will never be enough)

1. **Headers models** (not extracted at all — #2482).
2. **Operations/channels codegen**: no publisher/subscriber stubs, no router, no channel-address binding, no handler interfaces.
3. **Protocol bindings** (Kafka topics/keys, MQTT, WebSocket, HTTP): Modelina is transport-agnostic by design.
4. **Runtime validation**: zero generated validators; only a hardwired Gin `binding:"required"` tag.
5. **Middleware/interceptors, serialization beyond JSON tags** (no protobuf/Avro wire codecs, no XML/binary presets for Go).
6. **Go output quality gaps**: `AnonymousSchema_N` names for inline payloads (message names ignored — forces `$ref`-ing everything into components or accepting ugly names), no `time.Time`, no pointer/nullability control, un-gofmt'ed output, `AdditionalProperties` field on every struct, clunky `uint`-based enums.
7. **Go integration requires Node** — the fundamental architectural mismatch.

### "Reuse Modelina via CLI" vs "port ogen's JSON-Schema→Go engine"

| | Modelina via CLI | Port ogen's JSON-Schema→Go engine |
|---|---|---|
| Toolchain | Node ≥ 18 runtime inside a Go toolchain; sidecar or subprocess glue | **Pure Go**; single binary, `go install`-able, testable with `go test` |
| AsyncAPI nativeness | Native: 2.0→3.0, Avro/RAML payload formats, official project | None: ogen is OpenAPI-shaped; agen must parse AsyncAPI in Go and map message payloads/headers onto the schema engine |
| Output quality | Structs + json tags; gaps listed above | ogen's engine emits idiomatic Go: proper nullability/pointers, `time.Time` for date-time, null-wrappers, required/optional semantics, **generated validation code** with precise error paths |
| Extensibility for agen | CLI exposes 2 Go flags; deeper hooks need a Node sidecar script agen maintains | Full control: agen owns type mapping, naming, validation emission, and can add headers/operations/protocol code around it |
| Maintenance | Upstream maintained by AsyncAPI (v6 underway, 3-month majors); agen's glue is thin but pinned to output-breaking majors | Fork/port burden on agen; ogen's engine is battle-tested but not a library with a stable contract — porting means owning it |
| Risk | Node dep + output gaps (esp. `AnonymousSchema_N` naming, no headers) likely force post-processing that erodes the reuse benefit | Upfront engineering cost; AsyncAPI parsing in Go must be built anyway |

**Recommendation:** for a Go-first generator whose identity is "be the glue", Modelina is best treated as (a) a **reference architecture** (input-processor → meta-model → constrain → render-preset pipeline) and (b) an **optional** `--model-backend=modelina` escape hatch shelling out to the CLI for teams that value its multi-format/multi-language coverage. The **core path should be Go-native** — porting/reusing ogen's JSON-Schema→Go machinery gives agen idiomatic structs, real validation, `time.Time`, and headers support on day one, with zero Node dependency; the cost is owning the AsyncAPI-document→schema-IR mapping, which agen must own anyway to generate operations, bindings, and pub/sub stubs that Modelina will never produce.
