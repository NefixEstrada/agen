package streetlightspub_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/NefixEstrada/agen/runtime/broker"

	streetlightspub "github.com/NefixEstrada/agen/internal/integration/streetlights_pub"
)

// The streetlights_pub package is the client-only variant of the streetlights
// spec (generator.features.disable: [subscriber]): it must compile and publish
// without any handler surface.

func TestPublish(t *testing.T) {
	pub := &fakePublisher{}
	client := streetlightspub.NewClient(pub)

	err := client.SendLightCommand(context.Background(), &streetlightspub.LightCommand{
		Payload: streetlightspub.LightCommandPayload{
			Command: streetlightspub.NewOptLightCommandPayloadCommand(streetlightspub.LightCommandPayloadCommandOn),
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

func TestPublishOptions(t *testing.T) {
	pub := &fakePublisher{}
	client := streetlightspub.NewClient(pub)

	err := client.SendLightCommand(context.Background(), &streetlightspub.LightCommand{
		Payload: streetlightspub.LightCommandPayload{
			Command: streetlightspub.NewOptLightCommandPayloadCommand(streetlightspub.LightCommandPayloadCommandOff),
		},
	},
		streetlightspub.WithKey("1"),
		streetlightspub.WithHeader("requestId", "abc"),
	)
	require.NoError(t, err)

	require.Len(t, pub.messages, 1)
	out := pub.messages[0]
	require.Equal(t, "1", out.Key)
	require.Equal(t, "abc", out.Headers["requestId"])
}

func TestAddressConst(t *testing.T) {
	require.Equal(t, "streetlights.command", streetlightspub.LightCommandAddress)
}

// TestClientOnlySurface guards the feature configuration of this package:
// with the subscriber feature disabled only the client files are emitted.
func TestClientOnlySurface(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	files := make(map[string]bool, len(entries))
	for _, e := range entries {
		files[e.Name()] = true
	}

	require.True(t, files["aas_client_gen.go"], "client must be generated")
	require.False(t, files["aas_subscriber_gen.go"], "subscriber must not be generated with the subscriber feature disabled")
	require.False(t, files["aas_handlers_gen.go"], "handlers must not be generated with the subscriber feature disabled")
	require.False(t, files["aas_middleware_gen.go"], "middleware must not be generated with the subscriber feature disabled")
}

type fakePublisher struct {
	messages []broker.Outgoing
}

func (p *fakePublisher) Publish(ctx context.Context, out broker.Outgoing) error {
	p.messages = append(p.messages, out)
	return nil
}

func (p *fakePublisher) Close(ctx context.Context) error { return nil }
