# Vendored from ogen

Upstream: https://github.com/ogen-go/ogen
Version: v1.24.0
Source dir: `conv`

Copied verbatim (tests excluded); only import paths rewritten:
- `github.com/ogen-go/ogen/json`      → `github.com/NefixEstrada/agen/runtime/codec`
- `github.com/ogen-go/ogen/ogenregex` → `github.com/NefixEstrada/agen/runtime/ogenregex`

Re-diff with: `git diff --no-index <ogen>/json /home/nefix/dev/agen/runtime/conv` (modulo import rewrites).
