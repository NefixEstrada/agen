# Vendored from ogen

Upstream: https://github.com/ogen-go/ogen
Version: v1.24.0
Source dir: `validate`

Copied verbatim (tests excluded); only import paths rewritten:
- `github.com/ogen-go/ogen/json`      → `github.com/NefixEstrada/agen/runtime/codec`
- `github.com/ogen-go/ogen/ogenregex` → `github.com/NefixEstrada/agen/runtime/ogenregex`

Local additions:
- `ogen.go`: `ValidateWith` — upstream's `validators.tmpl` (vendored into `gen/_template`) emits
  `validate.ValidateWith` for object-level pluggable validation, but ogen v1.24.0 (and current
  main) never defines it, so generated object-level validators don't compile there. Added here
  with the same semantics as `Ogen`.

Re-diff with: `git diff --no-index <ogen>/json /home/nefix/dev/agen/runtime/validate` (modulo import rewrites).
