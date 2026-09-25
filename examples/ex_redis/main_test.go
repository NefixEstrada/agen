package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	demo "github.com/NefixEstrada/agen/examples/ex_redis/api"
)

// TestRedisDemoEndToEnd runs the demo flow (publish -> consume -> ack) against
// an in-memory Redis, wiring everything through the generated ogen-style
// constructors.
func TestRedisDemoEndToEnd(t *testing.T) {
	mr := miniredis.RunT(t)
	// Admin client, only to assert the XACK state of the stream.
	admin := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = admin.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	got := make(chan *demo.LightCommand, 1)
	srv, err := demo.NewServer(demoHandler{got: got},
		demo.WithAddr(mr.Addr()),
		demo.WithGroup("demo-app"),
	)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()

	select {
	case <-srv.Ready():
	case <-time.After(3 * time.Second):
		t.Fatal("server did not become ready")
	}

	c, err := demo.NewClient(mr.Addr())
	require.NoError(t, err)
	require.NoError(t, c.SendLightCommand(ctx, &demo.LightCommand{
		Payload: demo.LightCommandPayload{
			Command: demo.LightCommandPayloadCommandOn,
			SentAt:  time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC),
		},
	}))

	select {
	case msg := <-got:
		require.Equal(t, demo.LightCommandPayloadCommandOn, msg.Payload.Command)
		require.Equal(t, time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC), msg.Payload.SentAt)
	case <-time.After(3 * time.Second):
		t.Fatal("no message received")
	}

	// Successful dispatch XACKs the stream entry.
	require.Eventually(t, func() bool {
		pending, err := admin.XPending(ctx, demo.LightCommandAddress, "demo-app").Result()
		return err == nil && pending.Count == 0
	}, 3*time.Second, 50*time.Millisecond)

	cancel()
	<-done
	require.NoError(t, c.Close(context.Background()))
	require.NoError(t, srv.Close(context.Background()))
}
