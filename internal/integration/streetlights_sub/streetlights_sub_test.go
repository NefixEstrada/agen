package streetlightssub_test

import (
	"context"
	"os"
	"testing"

	"github.com/go-faster/errors"
	"github.com/stretchr/testify/require"

	"github.com/NefixEstrada/agen/runtime/agenerrors"

	streetlightssub "github.com/NefixEstrada/agen/internal/integration/streetlights_sub"
)

// The streetlights_sub package is the subscriber-only variant of the
// streetlights spec (generator.features.disable: [publisher]): it must
// compile and dispatch without any client surface.

// testIncoming is a minimal broker.Incoming for tests.
type testIncoming struct {
	topic       string
	contentType string
	headers     map[string]string
	body        []byte
	acked       bool
}

func (m *testIncoming) Topic() string          { return m.topic }
func (m *testIncoming) Key() []byte            { return nil }
func (m *testIncoming) ContentType() string    { return m.contentType }
func (m *testIncoming) Header(k string) string { return m.headers[k] }
func (m *testIncoming) HeaderNames() []string {
	keys := make([]string, 0, len(m.headers))
	for k := range m.headers {
		keys = append(keys, k)
	}
	return keys
}
func (m *testIncoming) Body() []byte    { return m.body }
func (m *testIncoming) Ack() error      { m.acked = true; return nil }
func (m *testIncoming) Nack(bool) error { return nil }

func TestDispatch(t *testing.T) {
	var got *streetlightssub.LightMeasured
	h := handlerFunc(func(ctx context.Context, msg *streetlightssub.LightMeasured) error {
		got = msg
		return nil
	})
	sub := streetlightssub.NewSubscriber(streetlightssub.NewSubscriberHandlers(h))

	raw := &testIncoming{
		topic:   "streetlights.2.light",
		headers: map[string]string{"requestId": "abc"},
		body:    []byte(`{"lumens":42}`),
	}
	require.NoError(t, sub.Dispatch(context.Background(), raw))
	require.True(t, raw.acked)

	require.NotNil(t, got)
	require.Equal(t, "abc", string(got.Headers.RequestId.Value))
	require.Equal(t, int(42), got.Payload.Lumens.Value)
}

func TestReceiveValidationFails(t *testing.T) {
	sub := streetlightssub.NewSubscriber(
		streetlightssub.NewSubscriberHandlers(
			handlerFunc(func(ctx context.Context, msg *streetlightssub.LightMeasured) error {
				return nil
			}),
		),
	)
	raw := &testIncoming{
		topic: "streetlights.2.light",
		body:  []byte(`{"lumens":"not a number"}`),
	}
	err := sub.Dispatch(context.Background(), raw)
	require.Error(t, err)
	var decodeErr *agenerrors.DecodeMessageError
	require.True(t, errors.As(err, &decodeErr))
	require.False(t, raw.acked)
}

func TestAddressBuilders(t *testing.T) {
	addr, err := streetlightssub.BuildLightMeasuredAddress("1")
	require.NoError(t, err)
	require.Equal(t, "streetlights.1.light", addr)

	_, err = streetlightssub.BuildLightMeasuredAddress("999")
	require.Error(t, err)
}

// TestSubscriberOnlySurface guards the feature configuration of this package:
// with the publisher feature disabled only the subscriber files are emitted.
func TestSubscriberOnlySurface(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	files := make(map[string]bool, len(entries))
	for _, e := range entries {
		files[e.Name()] = true
	}

	require.True(t, files["aas_subscriber_gen.go"], "subscriber must be generated")
	require.True(t, files["aas_handlers_gen.go"], "handlers must be generated")
	require.False(t, files["aas_client_gen.go"], "client must not be generated with the publisher feature disabled")
	require.False(t, files["aas_fakes_gen.go"], "fakes are not enabled for this variant")
	require.False(t, files["aas_unimplemented_gen.go"], "unimplemented is not enabled for this variant")
}

type handlerFunc func(ctx context.Context, msg *streetlightssub.LightMeasured) error

func (f handlerFunc) ReceiveLightMeasurement(ctx context.Context, msg *streetlightssub.LightMeasured) error {
	return f(ctx, msg)
}
