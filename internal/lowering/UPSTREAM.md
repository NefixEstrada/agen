# Vendored from ogen

Upstream: https://github.com/ogen-go/ogen
Version: v1.24.0
Source files: `gen/schema_gen.go`, `gen/schema_gen_primitive.go`, `gen/schema_gen_sum.go`,
`gen/schema_transform.go`, `gen/tstorage.go`, `gen/names.go`, `gen/generics.go`,
`gen/genctx.go`, `gen/imports.go`, `gen/errors.go` (subset), `gen/utils.go` (subset)

Copied + adapted (package `gen` → `lowering`). Local deltas:
- HTTP response/parameter/wtype storage removed from `tstorage.go`/`genctx.go`.
- `gen_equality*.go` and `gen_validators_unique.go` are vendored at the `gen` package level (`gen/gen_equality*.go`, `gen/gen_validators_unique.go`), driving the `ir.EqualityMethodSpec` types vendored here.
- `PascalSpecial`/`CamelSpecial` exported for the gen package.
- `Engine` API added (`engine.go`) driving `schemaGen` with one shared type storage.
- `DefaultImports` exported; import paths point at `agen/runtime/*` instead of `ogen/*`.
- `errors.go`: only `ErrNotImplemented` and `ErrFieldsDiscriminatorInference` kept.
- `utils.go`: only `unreachable`, `position`, `zapPosition` kept.

Re-diff per file with: `git diff --no-index <ogen>/gen/<file>.go internal/lowering/<file>.go`.
