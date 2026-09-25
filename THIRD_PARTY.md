# Third-party notices

## ogen — github.com/ogen-go/ogen (Apache-2.0)

agen reuses code from [ogen](https://github.com/ogen-go/ogen), Copyright The ogen Authors.

Vendored directories (each has its own `UPSTREAM.md` with origin version and local deltas):

| agen directory | ogen source | Disposition |
|---|---|---|
| `runtime/validate/` | `validate/` | copied |
| `runtime/codec/` | `json/` | copied |
| `runtime/ogenregex/` | `ogenregex/` | copied |
| `runtime/conv/` | `conv/` | copied |
| `internal/ir/` | `gen/ir` | copied + adapted (HTTP back-references replaced by AsyncAPI models) |
| `internal/lowering/` | `gen/schema_gen*.go`, `tstorage.go`, `names.go`, `generics.go`, `gen_equality*.go`, `genctx.go`, `imports.go` | copied + adapted |
| `internal/naming/` | `internal/naming` | copied |
| `gen/_template/` | `gen/_template` | copied subset + new agen templates (kept at the upstream-relative path for go:embed and re-diffing) |

ogen itself is a build-time-only dependency (public packages `jsonschema`, `jsonpointer`, `location`, `gen/genfs`):
generated code never imports it.
