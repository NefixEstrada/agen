# ogen (ogen-go/ogen) — Architecture Study for "agen"

> Provenance: research conducted 2026-09-23 (study phase) from a fresh shallow clone at
> `/tmp/agen-study/ogen` (commit `2569478`, September 2026). All paths are relative to the repo root.

---

## 1. Repo layout, module, license

```
github.com/ogen-go/ogen   (go.mod: go 1.25.0)
├── LICENSE               ⚠ Apache License 2.0  (NOT MIT)
├── ogen.go, spec.go, schema.go, dsl.go, security_scheme.go, ...   root package "ogen"
├── cmd/
│   ├── ogen/         main CLI code generator
│   ├── jschemagen/   standalone JSON Schema → Go types generator (!)
│   └── ogen-wasm/    WASM binding exposing parse+validate to JS
├── gen/              code generator (public)  + gen/ir, gen/genfs, gen/_template
├── openapi/          parsed OpenAPI AST      + openapi/parser
├── jsonschema/       JSON Schema parser (public, OpenAPI-independent!)
├── jsonpointer/      RFC 6901 pointers + $ref resolution infra
├── location/         file/line/col error locations + pretty CLI errors
├── json/             runtime: jx-based codecs for uuid/time/decimal/ip/mac
├── validate/         runtime: validation helpers used by generated .Validate()
├── uri/              runtime: query/cookie/header param (de)serialization
├── conv/             runtime: type conversions (time, etc.)
├── middleware/       runtime: Middleware interface (net/http)
├── ogenerrors/       runtime: typed decode/security errors
├── ogenregex/        runtime: regexp2 wrapper so generated code can use ECMA regexes
├── otelogen/         runtime: OpenTelemetry integration
├── http/ (as ht)     runtime: form/multipart/match helpers
├── sse/              runtime: server-sent events decoding
├── schemas/          SEPARATE go module (meta-schema of OpenAPI itself, yq-generated)
├── examples/         SEPARATE go module; committed generated code for real specs
├── internal/         naming, xmaps, xslices, bitset, httpcookie, urlpath, ogenzap,
│                     ogenversion, testutil, integration (committed generated code)
├── tools/            mkformattest (generates exhaustive format spec), sgcollector
│                     (crawls Sourcegraph for real-world specs, fuzzes ogen)
└── _testdata/        positive/ (~70 specs), negative/ (~10 error cases), examples/,
                      testtypes/, location/
```

**License caveat for agen:** ogen is **Apache-2.0**. Apache-2.0 code can be vendored into another project if you preserve copyright notices, attribute, and include the Apache-2.0 license text for the copied portions (standard NOTICE practice). Choosing Apache-2.0 for agen itself removes all friction.

**Notable go.mod deps:** `github.com/go-faster/jx` (runtime JSON — generated code emits jx calls, never reflection), `github.com/go-faster/yaml` (spec parsing, keeps `yaml.Node` locators), `github.com/go-faster/errors`, `golang.org/x/tools` (**goimports** for formatting + **go/packages** for `x-ogen-type` interface detection), `github.com/shopspring/decimal`, `github.com/google/uuid`, `github.com/dlclark/regexp2` (via `ogenregex`), `github.com/valyala/fasthttp` (benchmarks only), `go.opentelemetry.io/*`, `go.uber.org/zap`, `golang.org/x/sync` (errgroup), `github.com/yuin/goldmark` (markdown description rendering).

---

## 2. Full pipeline: OpenAPI spec → Go code

### Step-by-step data flow

1. **CLI entry** — `cmd/ogen/main.go`:
   - `loadConfig` auto-discovers `ogen.yml|ogen.yaml|.ogen.yml|.ogen.yaml` (strict `KnownFields(true)` decode into `gen.Options`).
   - `opts.SetLocation(specPath, ...)` (`gen/options.go`) reads the spec from disk or `http(s)://`/`file://` via `jsonschema.NewExternalResolver`, sets `Parser.File` (a `location.File` carrying raw bytes for error positions) and `Parser.RootURL`.

2. **Raw parse** — `ogen.Parse(data)` (root `ogen.go`): `yaml.Unmarshal` into `*ogen.Spec` (`spec.go`). `Spec` is the raw, 1:1 OpenAPI document model; every embedded object carries `jsonschema.OpenAPICommon` (inline: `Extensions map[string]yaml.Node` + `location.Locator`) so any error later can be pinned to line:col. Root also exposes a fluent builder DSL (`dsl.go`, e.g. `NewSpec().SetOpenAPI("3.1.0").AddOperation(...)`) used by tests/tools.

3. **Semantic parse** — `gen.NewGenerator(spec, opts)` (`gen/generator.go:83`) calls `parser.Parse(spec, parser.Settings{...})` (`openapi/parser/parser.go`) producing **`*openapi.API`** (`openapi/api.go`: `Version, Tags, Servers, Operations, Webhooks, Components{Schemas, Responses, Parameters, Examples, RequestBodies}, Info`).
   - The parser walks paths/webhooks/components, parses parameters (`parse_parameter.go`), media types (`parse_mediatype.go`), security (`parse_security.go`), and **delegates every schema to `jsonschema.Parser`** with a `RootResolver` over the root `yaml.Node`.
   - `$ref` handling: `jsonpointer.RefKey{Loc, Ptr}` keys; `jsonpointer.ResolveCtx` maintains a location stack for relative-ref resolution and detects infinite recursion; `openapi/parser/resolve.go` + lazy `refs` maps cache referenced components; `jsonschema/resolve.go` caches per-document resolvers fetched through `ExternalResolver.Get` (http/https/file).

4. **IR construction** — back in `NewGenerator` → `g.makeIR(api)`:
   - Per operation/webhook: `generateOperation` (`gen/gen_operation.go`) → `generateParameters` / `generateRequestBody` / `generateResponses` / `generateSecurity` → each funnels through **`Generator.generateSchema`** (`gen/gen_schema.go`) → **`schemaGen.generate`** (`gen/schema_gen.go:118`), the heart of schema→type lowering (details in §5).
   - Types are stored in `tstorage` (`gen/tstorage.go`) keyed by `jsonschema.Ref` + encoding; `genctx` has global (shared) + local (per-op) storages merged after each op.
   - `collectEqualitySpecs` finds types needing `Equal()`/`Hash()` for `uniqueItems`.
   - `g.route()` builds the router tree (`gen/router.go`, `gen/route_node.go`, `gen/route_tree.go`).

5. **Emission** — `(*Generator).WriteSource(fs FileSystem, pkgName string)` (`gen/write.go:250`):
   - Builds `TemplateConfig` (all IR collections + feature booleans), splits types vs interfaces.
   - Renders ~24 output files `oas_%s_gen.go` (schemas, json, uri, interfaces, parameters, handlers, request/response encoders/decoders, validators, middleware, server, client, cfg, servers, router, defaults, security, test_examples, faker, unimplemented, labeler, operations) from **text/template** templates embedded via `embed.FS` in `gen/_template/*.tmpl` and subdirs `json/`, `uri/`, `schema/`, `defaults/`, `faker/`.
   - Each template executes into a pooled buffer, then **`golang.org/x/tools/imports.Process`** (goimports = format + fix imports) runs, then the bytes go to the `gen.FileSystem` interface (`WriteFile(baseName, source)`); implementations: `genfs.FormattedSource` (disk writer, optional `go/format`), `genfs.CheckFS` (test: only `format.Source`-parses). Rendering is parallel (`errgroup` + pprof labels); on execute failure the raw buffer is dumped to `*.dump` for debugging.

**Key types by stage:** raw: `ogen.Spec`; semantic: `openapi.API`, `openapi.Operation`, `jsonschema.Schema`; IR: `gen/ir.Type`, `ir.Operation`, `ir.Field`, `ir.EnumVariant`, `ir.SumSpec`; emission: `gen.TemplateConfig`, `gen.FileSystem`.

---

## 3. Public vs internal; library usage

**Public (importable) packages:** root `ogen`, `gen`, `gen/ir`, `gen/genfs`, `openapi`, `openapi/parser`, `jsonschema`, `jsonpointer`, `location`, `json`, `validate`, `uri`, `conv`, `middleware`, `ogenerrors`, `ogenregex`, `otelogen`, `http`, `sse`. Plus the separate modules `schemas` and `examples`.

**Internal:** `internal/{naming, xmaps, xslices, bitset, httpcookie, urlpath, ogenzap, ogenversion, testutil, integration}`. Note: several *public* packages import internal ones (`location` → `internal/xmaps,xslices`; `jsonschema` → `internal/urlpath,xslices`; `gen` → `internal/naming,bitset,...`), so consuming ogen as a library requires the whole module (fine, since `internal/` is importable within the module).

**Yes, ogen is a library.** The public API surface:

```go
func ogen.Parse(data []byte) (*Spec, error)                     // ogen.go
func gen.NewGenerator(spec *ogen.Spec, opts gen.Options) (*gen.Generator, error)  // gen/generator.go:83
func (g *Generator) WriteSource(fs gen.FileSystem, pkgName string) error          // gen/write.go:250
func (g *Generator) Types() map[string]*ir.Type
func (g *Generator) Operations() []*ir.Operation
func (g *Generator) Webhooks() []*ir.Operation
func (g *Generator) API() *openapi.API
```

`cmd/ogen`, `cmd/jschemagen` are thin wrappers over exactly this API.

---

## 4. JSON Schema engine

Location: `jsonschema/` (public). **It has zero OpenAPI dependencies** — `openapi/parser` uses it, not the other way around.

- **Drafts supported:** none explicitly. This is an OpenAPI-3.0/3.1-flavored *subset* of JSON Schema. There is no `$schema`/`$id`/`$defs` vocabulary handling and no draft detection; version gating (3.0–3.2 only) happens in `openapi/parser/version.go`. OpenAPI 3.1 style `type: [string, "null"]` arrays are collapsed during unmarshal into `type: string` + `Nullable=true` (`jsonschema/raw_schema.go` custom `UnmarshalYAML/UnmarshalJSON` via `CollapseTypeNode`).
- **Parsed model:** `jsonschema.Schema` (`schema.go`) — type, format, contentEncoding/MediaType, properties/required, additionalProperties, patternProperties, items (single + tuple), enum/const, allOf/oneOf/anyOf, discriminator, full validator set (min/max/exclusive, multipleOf, length, pattern, items, uniqueItems, properties counts), nullable, default, examples, plus `location.Pointer` for error reporting.
- **Raw model:** `jsonschema.RawSchema` (`raw_schema.go`) with typed custom unmarshaling for `Num`, `Enum`, `Const`, `Default`, `Example` (`num.go`, `enum.go`, `const.go`, `raw_value.go`); numbers kept as `big.Rat`-ish `Num` so arbitrary precision multipleOf works.
- **`$ref` resolution:** `jsonpointer.RefKey{Loc,Ptr}`. `RootResolver` (`resolver.go`) resolves pointers against a cached root `yaml.Node`; `Parser` (`parser.go`) keeps `refcache` (dedup) and per-URL `resolver`s (`resolve.go`): external documents fetched via `ExternalResolver.Get` (http/https/file; `external.go`), parsed to `yaml.Node`, then `NewRootResolver`. `jsonpointer.ResolveCtx` (`jsonpointer/resolve_ctx.go`) carries a location *stack* so relative pointers inside external docs resolve against the right base, and doubles as recursion detection (`AddKey/Delete`).
- **Type inference:** `Settings.InferTypes` — `jsonschema/infer.go` can infer schema type from example/default values (e.g., `items` present → array).
- **Formats:** not in `jsonschema`; the format→Go-type table lives in **`gen.TypeFormatMapping()`** (`gen/schema_gen_primitive.go:171`): integers int8–uint64 + `unix`/`unix-seconds|nano|micro|milli`; number float/double/`decimal`; strings `byte`,`base64`,`date`,`time`,`date-time`,`http-date`→`time.Time`, `duration`, `uuid`, `mac`, `ip/ipv4/ipv6`→`netip.Addr`, `uri`→`url.URL`, `email/password/binary/hostname`→string, plus non-standard int/uint/float formats and `decimal`. Unknown format falls back to the type default. `x-ogen-time-format` overrides the time layout. `x-ogen-type` (`gen/ir/external.go`) loads an arbitrary external Go type and uses **`golang.org/x/tools/go/packages`** to type-check which interfaces it implements (ogen-native jx, std JSON, Text, Binary) — that decides the emitted encode/decode path.
- **Nullable:** end-to-end story: `schema.Nullable` → `boxType(t, GenericVariant{Optional, Nullable})` (`gen/generics.go:57`). Arrays/slices get `NilSemantic` in place (`NilOptional` vs `NilNull` — `gen/ir/nil_semantic.go` distinguishes "absent" vs "JSON null"); other types get wrapped in a generated generic `Opt…` wrapper (`ir.KindGeneric`, e.g. `OptString`, `OptAnyTestAnyMap`).
- **Validation codegen:** `gen/_template/validators.tmpl` (323 lines) emits `func (s T) Validate() error` per type, delegating to the public runtime package **`validate`** (`validate/array.go`, `string.go`, `int.go`, `float.go`, `decimal.go`, `object.go` — e.g. `(validate.Array{MinLength:…}).ValidateLength(len(x))`), with regexes hoisted into package-level `ogenregex` vars (`TemplateConfig.RegexStrings()`). Cross-type constraints (pattern on numbers, etc., on by default) are interpreted at parse time. Custom runtime validators: `x-ogen-validate` map + `validate/ogen.go` `OgenValidatorRegistry` (register-by-name at runtime). Complex `uniqueItems` generates `Equal()`/`Hash()` methods + `validateUniqueT()` (`gen/gen_equality*.go`, `gen_validators_unique.go`).

---

## 5. Type & code generation

- **IR kinds** (`gen/ir/type.go:16`): `Primitive, Array, Map, Alias, Const, Enum, Struct, Pointer, Interface, Generic, Sum, Any, Stream`. `ir.Type` carries Doc, Name, `Schema *jsonschema.Schema` back-ref, `NilSemantic`, `Validators`, `Features` (`"json"`, `"uri"` — a type used in both body and query gets both codec sets), `Implementations/Implements` for interface relationships.
- **Derivation rules** (`gen/schema_gen.go` `generate`/`generate2`, `schema_gen_sum.go`, `schema_gen_primitive.go`, `schema_transform.go`):
  - object → struct; `additionalProperties` → `map[string]T`; `patternProperties` → map with `MapPattern` key validation
  - array → slice; tuple (`items: [...]`) → struct with positional fields (`Tuple`)
  - `allOf` → **flattened** into one struct (`flattenAllOfSchema`) with overlap checks
  - `oneOf`/`anyOf` → `KindSum`; `SumSpec` supports explicit `discriminator`+mapping, **unique-field** discrimination, **type-based** (jx.Type) discrimination, **value-based** (enum) discrimination, and a default mapping; sums become Go interfaces (`KindInterface`) with variant structs implementing them (`visit*`/`encode*` switch dispatch)
  - enum → `KindEnum` (int/string only; formatted-string enums restricted); const → `KindConst`
  - optional/nullable → `boxType` generics (`OptT` wrappers, `Get/Set` accessors) or NilSemantic for slices
  - recursion safety: depth limit (default 1000, `schemaDepthError`), `checkStructRecursions` rejects unboxable recursive structs
- **Emission mechanism:** **`text/template` only** — no go/ast building, no dave/dst. Templates in `gen/_template/` (`embed.FS`, parsed once lazily with a rich `templateFunctions()` + `ir/template_helpers.go`). Post-pass = goimports (`imports.Process`), which is what keeps imports/aliases correct (`gen/imports.go` pre-seeds the alias map). Code style: hand-written-Go-looking output, jx streaming encoders/decoders (e.g., `e.ObjStart()/e.FieldStart(...)`), errors wrapped with `go-faster/errors`.
- **Naming:** `gen/names.go` `nameGen` — splits camelCase/snake/kebab, maps special chars (`+`→`Plus`, `/`→`Slash`, …), prefixes leading digits with `R`, validates with `go/token.IsIdentifier`; ref names from last pointer segment (`cleanRef`); initialism rules from `internal/naming/rules.go` (staticcheck-style list, incl. `OAuth2`, `SHA256`), user-overridable via `initialisms:` config (with `inherit` sentinel) and `naming/camel_initialisms` feature; `x-ogen-name`/`x-ogen-properties.name` overrides. Enum variant naming has multiple strategies (`namer().enumVariantNameGen`).
- **Error types generated:** per-op decode errors use runtime `ogenerrors` (`DecodeParamsError`, `DecodeParamError`, `DecodeRequestError`, `DecodeBodyError`, `SecurityError`, `ErrorHandler`); "convenient errors" feature auto-detects a common error response pattern and generates an `Error` interface + `StatusCode()`/`Unwrap()`; generator-side errors in `gen/errors.go` (`ErrNotImplemented` → `ignore_not_implemented`, `ErrUnsupportedContentTypes`, `ErrParseSpec`, `ErrBuildRouter`, `ErrGoFormat`, `ErrFieldsDiscriminatorInference`).

---

## 6. Configuration & extension points

`ogen.yml` (auto-discovered names above, strict parsing) → `gen.Options` (`gen/options.go`):

- `parser:` `infer_types`, `allow_remote`, `depth_limit` (default 1000), `authentication_schemes`, `allow_cross_type_constraints` (default true), `disallow_duplicate_method_paths`
- `generator:` `features: {enable: [], disable: [], disable_all: bool}`, `filters: {path_regex, methods}`, `ignore_not_implemented: []` (+ Go-side `NotImplementedHook`), `convenient_errors: auto|on|off`, `content_type_aliases` (e.g. `text/json: application/json`), `wildcard_content_type_default`, `initialisms: [inherit, …]`
- `expand:` writes a fully-$ref-expanded spec to disk

**Feature flags** (`gen/features.go`): `paths/client`, `paths/server`, `webhooks/client`, `webhooks/server`, `client/security/reentrant`, `client/request/options`, `client/request/validation`, `client/editors`, `server/response/validation`, `ogen/otel`, `ogen/unimplemented` (stub handler), `debug/example_tests` (generates example-based `_gen_test.go` + faker), `naming/camel_initialisms`.

**x-ogen extensions** (grep-verified): `x-ogen-name`, `x-ogen-type`, `x-ogen-time-format`, `x-ogen-properties` (per-property name overrides), `x-ogen-validate` (custom validators), `x-ogen-operation-group`, `x-ogen-server-name`, `x-ogen-custom-security`, `x-ogen-json-streaming`, `x-ogen-raw-response`, `x-ogen-sse-event-shape`, and third-party `x-oapi-codegen-extra-tags`.

**Hooks/filters:** `NotImplementedHook func(name string, err error)`, `Filters.accept(op)` (path regex + methods), feature-driven file enablement matrix in `WriteSource`.

---

## 7. Runtime packages the generated code imports

From `gen/imports.go` (default import table) and verified in `internal/integration/sample_api/*.go`:

| Package | Role | AsyncAPI reuse |
|---|---|---|
| `github.com/ogen-go/ogen/validate` | length/range/pattern/object validation helpers + custom validator registry | direct reuse |
| `ogen/json` | jx codecs for uuid, time (multi-layout), decimal, ip, mac, duration, unix | direct reuse |
| `ogen/ogenregex` | `regexp2` wrapper type for spec patterns | direct reuse |
| `ogen/conv` | value conversions (e.g., time ↔ string) | direct reuse |
| `ogen/ogenerrors` | typed decode errors — **HTTP-coupled** (`Code() int`, `net/http`) | adapt (message/protocol errors) |
| `ogen/uri` | query/cookie/header param + object param (de)serialization | partially reusable for message headers |
| `ogen/middleware` | `Middleware` iface + `Parameters` — net/http | rewrite per broker |
| `ogen/http` (ht) | form/multipart/match | HTTP-specific |
| `ogen/otelogen` | tracer/meter/semconv helpers for generated instrumentation | mostly reusable pattern |
| `ogen/sse` | SSE event decoding | HTTP-specific |
| external | `go-faster/jx`, `go-faster/errors`, `google/uuid`, `shopspring/decimal`, `go.uber.org/multierr`, otel | direct reuse |

---

## 8. Testing approach

- **Positive golden-ish:** root `gen_test.go` walks `_testdata/positive` (~70 specs) and `_testdata/examples` (k8s, github, telegram, openai, firecracker…); generates with `genfs.CheckFS` — i.e., asserts output **parses under `go/format`**, not byte-for-byte golden compare. Per-file skip sets assert that every `ignore_not_implemented` entry was actually needed.
- **Negative:** `_testdata/negative/*/*.json` — asserts `gen.NewGenerator` returns an error (parser-level failures must live in `openapi/parser/_testdata`), printing via `location.PrintPrettyError`.
- **Location tests:** `_testdata/location/` drives line/col error output (`location_test.go`).
- **Integration:** `internal/integration/` — **committed generated code** (e.g., `sample_api/oas_*_gen.go`), regenerated by `go:generate` directives in `internal/integration/generate.go` (runs `cmd/ogen` with per-case `_config/*.yml`); the packages must compile and pass extensive behavioral tests (JSON round-trips, params, security, CORS, techempower bench).
- **Examples module:** `examples/` (own go.mod) — committed generated code for real-world specs, `go generate`-managed.
- **Tooling:** `tools/mkformattest` generates a spec exercising *every* format→type mapping; `tools/sgcollector` crawls Sourcegraph for real-world OpenAPI specs and reports ogen crashes (continuous fuzz-ish QA).
- **CI** (`.github/workflows/`): `ci.yml` (matrix ubuntu/macos/windows × amd64, 386, `-race`), `lint.yml` (golangci-lint), `coverage.yml` (codecov), `codeql-analysis.yml`, `commitlint.yml`, `image_build.yml`, `tidy-autocommit.yml`. Makefile: `make generate` (`go generate ./...`), `commit_gen` commits regenerated code — generated code is always kept compiling in-tree.

---

## 9. Reusability assessment for AsyncAPI (the important part)

### Verified intra-module dependency graph (`go list`)

```
location        → internal/xmaps, internal/xslices            [GENERIC]
jsonpointer     → location                                     [GENERIC]
json            → (none)                                       [GENERIC]
ogenregex       → (none)                                       [GENERIC]
validate        → ogenregex                                    [GENERIC]
conv            → json                                         [GENERIC]
jsonschema      → json, jsonpointer, location, ogenregex,
                  internal/urlpath, internal/xslices           [GENERIC — NO openapi dep]
openapi         → jsonpointer, jsonschema, location            [OAS AST]
openapi/parser  → ogen(root), openapi, jsonschema, jsonpointer,
                  location, uri, internal/httpcookie, xmaps, xslices   [OAS/HTTP]
gen/ir          → jsonschema, ogenregex, openapi, validate,
                  internal/{naming, bitset, xmaps, xslices}    [MOSTLY GENERIC; openapi back-refs]
gen             → ogen, openapi, openapi/parser, gen/ir, json, jsonschema,
                  location, ogenregex, internal/{naming,bitset,urlpath,xmaps,xslices}  [MIXED]
gen/genfs       → (none)                                       [GENERIC]
uri             → validate, internal/httpcookie                [HTTP-ish]
middleware      → openapi                                      [HTTP]
ogenerrors      → http, openapi, validate                      [HTTP]
otelogen, http, sse → (none)                                   [HTTP]
```

Direct answers: **`jsonschema` does NOT depend on the parser** (dependency is the reverse). **`gen` DOES depend on HTTP/OpenAPI-specific packages** (`openapi`, `openapi/parser`, root `ogen`), mainly through `Generator`/operation machinery — but its schema-lowering core (`schema_gen*.go`, `tstorage`, `names`, `generics`) is separable. `gen/ir` depends on `openapi` only for back-references (`Operation.Spec *openapi.Operation`, `Parameter.Spec`, `ir/security.go`).

### Tier 1 — copy nearly as-is (pure, zero OpenAPI knowledge)

- `location/` + `internal/xmaps`, `internal/xslices` — error positions, pretty CLI errors (`PrintPrettyError`). AsyncAPI is YAML+JSON-Schema too, so this drops in unchanged.
- `jsonpointer/` — RFC 6901 + `RefKey` + `ResolveCtx` recursion detection.
- `jsonschema/` — the whole JSON Schema → parsed-schema engine, incl. external refs, inference, cross-type constraints. Bring `internal/urlpath` along. This is the single biggest win.
- `json/`, `ogenregex/`, `validate/`, `conv/` — runtime codecs + validation. Copy + re-point import paths (or keep as ogen dependency if you prefer depending on ogen at runtime — but vendoring decouples release cadence).
- `internal/naming` (initialism ruleset), `internal/bitset`, `gen/genfs`.

### Tier 2 — copy with adaptation (surgical edits, keep structure)

- **`gen/ir`** — keep `type.go`, `field.go`, `enum.go`, `primitive.go`, `generics.go`, `nil_semantic.go`, `validation.go`, `type_features.go`, `json.go`, `equal*`, `default.go`, `template_helpers.go`; strip/replace `operation.go`'s `Spec *openapi.Operation` + PathParts, `params.go`'s `openapi.Parameter` back-ref, `security.go`'s `openapi.SecurityScheme`, `responses.go`/`media.go` HTTP response/content semantics. If agen's IR "channel operation" mirrors ogen's `ir.Operation` minus HTTP, most template helpers keep working.
- **`gen` schema core** — `schema_gen.go`, `schema_gen_sum.go`, `schema_gen_primitive.go` (the `TypeFormatMapping` table), `schema_transform.go`, `gen_schema.go`, `tstorage.go`, `names.go`, `generics.go`, `genctx.go`, `gen_equality*.go`, `gen_validators_unique.go`, `reduce.go` (partially). Drop: `gen_operation.go`, `gen_parameters.go`, `gen_request_body.go`, `gen_responses.go`, `gen_security.go`, `gen_headers.go`, `gen_contents.go` (HTTP content-type negotiation incl. SSE), `router*.go`, `route_tree.go`, `responseencode.go`, `gen_server.go`.
- **Templates** — reuse `schema/*`, `json/*`, `validators.tmpl`, `defaults*`, `interfaces.tmpl`, `faker/*`, `cfg.tmpl`, `godoc.tmpl`, `globals.tmpl`, and the `ir` helper methods they call; write new templates for broker handlers/publishers/consumers. Note templates call `ir` methods directly, so keeping `gen/ir` API-compatible minimizes template edits.
- **`uri/`** — header (de)serializers are reusable for AMQP/Kafka/HTTP-message headers; query/cookie parts are not needed.
- **`otelogen`** — pattern reusable for broker instrumentation (rename semconv keys).

### Tier 3 — HTTP/OpenAPI-specific (rewrite; use as reference only)

- Root `spec.go`/`schema.go`/`security_scheme.go` (OpenAPI document model) → write an AsyncAPI 2.x/3.0 document model (note AsyncAPI reuses JSON Schema draft-07 shapes, which ogen's `RawSchema` already largely covers).
- `openapi/` + `openapi/parser/` → new `asyncapi/parser` producing an analogous parsed API struct. **Copy the architectural pattern**: parser owns document semantics, delegates 100% of schema work to `jsonschema.Parser` with a `RootResolver`, uses `jsonpointer.ResolveCtx` + lazy ref caches.
- `http/`, `sse/`, `middleware/` (net/http), `ogenerrors` (HTTP status codes), router machinery, request/response encoder templates, security-scheme gen (OAuth2/API-key over HTTP).

### Practical copy recipe (minimum viable vendoring set)

```
location/, jsonpointer/, jsonschema/, json/, ogenregex/, validate/, conv/,
internal/{naming, xmaps, xslices, bitset, urlpath},
gen/{ir minus openapi back-refs, schema core files, _template subset}, gen/genfs,
uri/ (header parts), otelogen (optional)
```

`x-ogen-type` support drags in `golang.org/x/tools/go/packages` (heavy but build-time only). The `jschemagen` cmd is existence proof the schema pipeline runs standalone — worth cloning as the skeleton of agen's first CLI (`cmd/agen` = jschemagen + AsyncAPI document parse + your broker templates).

---

## 10. Other notes for a sibling project

- **Two extra modules** (`schemas/`, `examples/`) keep generated artifacts and tool deps out of the main module graph — a pattern worth copying (agen will accumulate generated test code fast).
- **Formatting infra:** goimports (`x/tools/imports.Process`) at generation time; `go/format` only in `genfs.CheckFS`/optional `FormattedSource.Format`. No dave/dst anywhere; templates + goimports is deliberately simple.
- **Keeping generated code compiling:** commit integration/examples generated output, regenerate via `go:generate` + `make generate`/`commit_gen`, CI compiles it on 3 OSes + race; `sgcollector` stress-tests against wild specs; `mkformattest` guarantees every format mapping has a codec round-trip test.
- **Error UX:** everything threaded with `location.Pointer`; CLI prints colored line/col excerpts (`location.PrintPrettyError`); `ErrNotImplemented` becomes actionable "add ignore_not_implemented" advice — a very user-friendly pattern to replicate.
- **Webhooks ≠ AsyncAPI:** ogen's webhooks are still HTTP request/response handlers (a webhook server is an `http.Handler` receiving POSTs); the abstraction does not carry over — agen needs a genuinely broker-shaped handler/publisher model. AsyncAPI's per-protocol bindings are the analogue of ogen's `uri`/`gen_contents` content-negotiation layer.
- **Conventions worth stealing:** `FileSystem` abstraction for output (enables CheckFS-style tests), feature flags driving per-file generation, `NotImplementedHook` for library consumers, buffer pooling + errgroup parallelism, `x-ogen-*` extension namespace, and the strict-config-file-plus-CLI-override precedence model.
