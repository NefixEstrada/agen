package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	redisruntime "github.com/NefixEstrada/agen/runtime/redis"

	demo "github.com/NefixEstrada/agen/examples/redis/api"
)

// TestRedisDemoEndToEnd runs the demo flow (publish -> consume -> ack) against
// an in-memory Redis.
func TestRedisDemoEndToEnd(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	got := make(chan *demo.LightCommand, 1)
	sub := demo.NewSubscriber(demo.NewSubscriberHandlers(
		handlerFunc(func(ctx context.Context, msg *demo.LightCommand) error {
			got <- msg
			return nil
		}),
	))

	consumer, err := redisruntime.NewConsumer(redisruntime.ConsumerConfig{
		Client:    client,
		Addresses: []string{demo.LightCommandAddress},
		Group:     "demo-app",
		Block:     100 * time.Millisecond,
	})
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx, sub) }()

	publisher, err := redisruntime.NewPublisher(redisruntime.PublisherConfig{Client: client})
	require.NoError(t, err)
	c := demo.NewClient(publisher)
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
		pending, err := client.XPending(ctx, demo.LightCommandAddress, "demo-app").Result()
		return err == nil && pending.Count == 0
	}, 3*time.Second, 50*time.Millisecond)

	cancel()
	<-done
	require.NoError(t, publisher.Close(ctx))
}
