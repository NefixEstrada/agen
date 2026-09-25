# Vendored from ogen

Upstream: https://github.com/ogen-go/ogen
Version: v1.24.0
Source dir: `gen/ir`

Copied + adapted. Local deltas:
- HTTP machinery removed: `operation.go`, `params.go`, `responses.go`, `security.go`, `media.go`, `server.go`, `tag.go` dropped.
- `operation.go` replaced by the agen pub/sub model: `Operation` (send/receive), `Channel`, `ChannelParam`, `Message`, `Server`.
- `field.go`: HTTP form-parameter helpers (`parameters`, `FormParameters`, `FileParameters`) removed.
- `tag.go`: `Form *openapi.Parameter` field removed (no HTTP back-references).
- `encoding.go` added: minimal `Encoding` type (JSON only) extracted from upstream `media.go`.
- SSE machinery in `type.go`/`template_helpers.go` left in place but unused.

Re-diff with: `git diff --no-index <ogen>/gen/ir internal/ir`.
