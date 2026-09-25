// Command redis-demo is a runnable example of agen's Redis runtime: it
// generates the typed package from asyncapi.yaml (see agen.yml), then
// publishes light commands to Redis and receives them back through a typed
// subscriber.
//
// Start a Redis server (e.g. docker run -p 6379:6379 redis) and run:
//
//	go run ./examples/redis
//
// In streams mode (default) messages go through a consumer group (XADD →
// XREADGROUP → XACK). With -mode pubsub they use Redis Pub/Sub instead
// (fire-and-forget, no groups).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	redisruntime "github.com/NefixEstrada/agen/runtime/redis"

	demo "github.com/NefixEstrada/agen/examples/ex_redis/api"
)

func main() {
	var (
		addr = flag.String("addr", "localhost:6379", "Redis address")
		mode = flag.String("mode", "streams", "streams or pubsub")
		n    = flag.Int("n", 3, "number of commands to publish")
	)
	flag.Parse()

	if err := run(*addr, *mode, *n); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %+v\n", err)
		os.Exit(1)
	}
}

func run(addr, mode string, n int) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	got := make(chan *demo.LightCommand, n)
	sub := demo.NewSubscriber(demo.NewSubscriberHandlers(
		handlerFunc(func(ctx context.Context, msg *demo.LightCommand) error {
			fmt.Printf("received command=%s sentAt=%s\n", msg.Payload.Command, msg.Payload.SentAt.Format(time.RFC3339))
			got <- msg
			return nil
		}),
	))

	consumer, err := redisruntime.NewConsumer(redisruntime.ConsumerConfig{
		Addr:      addr,
		Mode:      redisruntime.Mode(mode),
		Addresses: []string{demo.LightCommandAddress},
		Group:     "demo-app",
	})
	if err != nil {
		return err
	}

	consumerDone := make(chan error, 1)
	go func() { consumerDone <- consumer.Run(ctx, sub) }()

	// Wait for the subscription/group to be established: publishing earlier
	// can lose messages in pubsub mode.
	select {
	case <-consumer.Ready():
	case <-time.After(5 * time.Second):
		return fmt.Errorf("consumer did not become ready")
	}

	publisher, err := redisruntime.NewPublisher(redisruntime.PublisherConfig{
		Addr: addr,
		Mode: redisruntime.Mode(mode),
	})
	if err != nil {
		return err
	}
	client := demo.NewClient(publisher)

	fmt.Printf("publishing %d light commands to %s (mode=%s)\n", n, demo.LightCommandAddress, mode)
	for i := 0; i < n; i++ {
		command := demo.LightCommandPayloadCommandOn
		if i%2 == 1 {
			command = demo.LightCommandPayloadCommandOff
		}
		if err := client.SendLightCommand(ctx, &demo.LightCommand{
			Payload: demo.LightCommandPayload{
				Command: command,
				SentAt:  time.Now().UTC(),
			},
		}); err != nil {
			return err
		}
	}

	for i := 0; i < n; i++ {
		select {
		case <-got:
		case <-time.After(10 * time.Second):
			return fmt.Errorf("timed out waiting for message %d", i+1)
		case err := <-consumerDone:
			return fmt.Errorf("consumer stopped: %w", err)
		}
	}

	stop()
	<-consumerDone
	return publisher.Close(ctx)
}

type handlerFunc func(ctx context.Context, msg *demo.LightCommand) error

func (f handlerFunc) ReceiveLightCommand(ctx context.Context, msg *demo.LightCommand) error {
	return f(ctx, msg)
}
