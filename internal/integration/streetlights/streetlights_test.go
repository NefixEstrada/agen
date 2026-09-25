package streetlights_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-faster/errors"
	"github.com/stretchr/testify/require"

	"github.com/NefixEstrada/agen/runtime/agenerrors"
	"github.com/NefixEstrada/agen/runtime/broker"

	streetlights "github.com/NefixEstrada/agen/internal/integration/streetlights"
)

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

func TestAddressBuilders(t *testing.T) {
	addr, err := streetlights.BuildLightMeasuredAddress("1")
	require.NoError(t, err)
	require.Equal(t, "streetlights.1.light", addr)

	_, err = streetlights.BuildLightMeasuredAddress("999")
	require.Error(t, err)

	require.Equal(t, "streetlights.command", streetlights.LightCommandAddress)
}

func TestReceiveDispatch(t *testing.T) {
	var (
		got *streetlights.LightMeasured
	)
	h := handlerFunc(func(ctx context.Context, msg *streetlights.LightMeasured) error {
		got = msg
		return nil
	})
	sub := streetlights.NewSubscriber(streetlights.NewSubscriberHandlers(h))

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
	sub := streetlights.NewSubscriber(
		streetlights.NewSubscriberHandlers(
			handlerFunc(func(ctx context.Context, msg *streetlights.LightMeasured) error {
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

func TestPublish(t *testing.T) {
	pub := &fakePublisher{}
	client := streetlights.NewClient(pub)

	err := client.SendLightCommand(context.Background(), &streetlights.LightCommand{
		Payload: streetlights.LightCommandPayload{
			Command: streetlights.NewOptLightCommandPayloadCommand(streetlights.LightCommandPayloadCommandOn),
		},
	})
	require.NoError(t, err)

	require.Len(t, pub.messages, 1)
	out := pub.messages[0]
	require.Equal(t, "streetlights.command", out.Topic)
	require.Equal(t, "application/json", out.ContentType)

	var body struct {
		Command string `json:"command"`
	}
	require.NoError(t, json.Unmarshal(out.Body, &body))
	require.Equal(t, "on", body.Command)
}

func TestMiddlewareChain(t *testing.T) {
	var order []string
	h := handlerFunc(func(ctx context.Context, msg *streetlights.LightMeasured) error {
		order = append(order, "handler")
		return nil
	})
	mw := func(name string) broker.Middleware {
		return func(next broker.Handler) broker.Handler {
			return func(ctx context.Context, raw broker.Incoming) error {
				order = append(order, name)
				return next(ctx, raw)
			}
		}
	}
	sub := streetlights.NewSubscriber(streetlights.NewSubscriberHandlers(h), mw("first"), mw("second"))

	raw := &testIncoming{
		topic: "streetlights.2.light",
		body:  []byte(`{"lumens":42}`),
	}
	require.NoError(t, sub.Dispatch(context.Background(), raw))

	// Middlewares apply left to right (the first is the outermost), then the
	// typed handler runs and the message is acked.
	require.Equal(t, []string{"first", "second", "handler"}, order)
	require.True(t, raw.acked)
}

func TestMiddlewareSeesHandlerError(t *testing.T) {
	errBoom := errors.New("boom")
	h := handlerFunc(func(ctx context.Context, msg *streetlights.LightMeasured) error {
		return errBoom
	})
	var seen error
	mw := func(next broker.Handler) broker.Handler {
		return func(ctx context.Context, raw broker.Incoming) error {
			err := next(ctx, raw)
			seen = err
			return err
		}
	}
	sub := streetlights.NewSubscriber(streetlights.NewSubscriberHandlers(h), mw)

	raw := &testIncoming{
		topic: "streetlights.2.light",
		body:  []byte(`{"lumens":42}`),
	}
	err := sub.Dispatch(context.Background(), raw)
	require.ErrorIs(t, err, errBoom)
	require.ErrorIs(t, seen, errBoom)
	require.False(t, raw.acked)
}

type handlerFunc func(ctx context.Context, msg *streetlights.LightMeasured) error

func (f handlerFunc) ReceiveLightMeasurement(ctx context.Context, msg *streetlights.LightMeasured) error {
	return f(ctx, msg)
}

type fakePublisher struct {
	messages []broker.Outgoing
}

func (p *fakePublisher) Publish(ctx context.Context, out broker.Outgoing) error {
	p.messages = append(p.messages, out)
	return nil
}

func (p *fakePublisher) Close(ctx context.Context) error { return nil }
