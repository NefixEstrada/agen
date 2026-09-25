package gen_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	parser "github.com/NefixEstrada/agen/asyncapi/parser"
	"github.com/NefixEstrada/agen/gen"
)

// memFS captures generated files.
type memFS map[string]string

func (m memFS) WriteFile(name string, source []byte) error {
	m[name] = string(source)
	return nil
}

func generateTo(t *testing.T, path string, opts gen.Options) memFS {
	t.Helper()
	api := parseWithAliasing(t, path)
	g, err := gen.NewGenerator(api, opts)
	require.NoError(t, err)
	fs := memFS{}
	require.NoError(t, g.WriteSource(fs, "extapi"))
	return fs
}

func joined(t *testing.T, fs memFS) string {
	t.Helper()
	names := make([]string, 0, len(fs))
	for k := range fs {
		names = append(names, k)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		b.WriteString(fs[n])
	}
	return b.String()
}

func TestXExtensions(t *testing.T) {
	api := parseWithAliasing(t, "../_testdata/positive/extensions/spec.yaml")
	g, err := gen.NewGenerator(api, gen.Options{})
	require.NoError(t, err)
	fs := memFS{}
	require.NoError(t, g.WriteSource(fs, "extapi"))
	out := joined(t, fs)

	// x-agen-name renames the message envelope.
	require.Contains(t, out, "type DeviceReading struct")
	// x-agen-time-format maps the date-time property to a custom Go layout.
	require.Contains(t, out, `json.DecodeTimeFormat(d, "2006-01-02 15:04:05")`)
	require.Contains(t, out, `json.EncodeTimeFormat(e, s.MeasuredAt, "2006-01-02 15:04:05")`)
	require.Contains(t, out, "MeasuredAt time.Time")
	// format: unix maps to time.Time via unix-seconds string decoding.
	require.Contains(t, out, "json.DecodeStringUnixSeconds")
	require.Contains(t, out, `json:"seenAt"`)
	// x-agen-validate: property-level validators get the field value...
	require.Contains(t, out, `validate.Ogen("corporate", s.Email, true)`)
	require.Contains(t, out, `validate.Ogen("reservedHandles", s.Handle, []interface{}{"admin", "root"})`)
	// ...object-level ones the whole struct.
	require.Contains(t, out, `validate.ValidateWith("crossField", s, map[string]interface{}{"separator": "-"})`)
}

// parseWithAliasing mirrors the CLI: alias x-agen-* before decoding.
func parseWithAliasing(t *testing.T, path string) *parser.API {
	t.Helper()
	api := parseSpecRaw(t, path, true)
	return api
}

func TestOperationReply(t *testing.T) {
	fs := generateTo(t, "../_testdata/positive/reply/spec.yaml", gen.Options{})
	out := joined(t, fs)

	// The reply becomes a typed publish surface on the client.
	require.Contains(t, out, "func (c *Client) ReceivePingReply(ctx context.Context, msg *Pong, opts ...PublishOption) error")
	require.Contains(t, out, "ResponsesAddress")
	// The receive operation itself is unchanged.
	require.Contains(t, out, "type ReceivePingHandler interface {")
}
