// Command redis-demo is a runnable example of agen's ogen-style generated
// API: the typed package is generated from asyncapi.yaml (see
// examples/generate.go), then the generated NewServer/NewClient wire the
// Redis runtime internally — user code never imports agen/runtime, exactly
// like ogen's generated HTTP server/client.
//
// Start a Redis server (e.g. docker run -p 6379:6379 redis) and run:
//
//	go run ./examples/ex_redis
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

	demo "github.com/NefixEstrada/agen/examples/ex_redis/api"
)

func main() {
	var (
		addr = flag.String("addr", "localhost:6379", "Redis address")
		mode = flag.String("mode", "streams", "streams or pubsub")
		n    = flag.Int("n", 3, "number of commands to publish")
	)
	flag.Parse()

	if err := run(*addr, demo.Mode(*mode), *n); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %+v\n", err)
		os.Exit(1)
	}
}

func run(addr string, mode demo.Mode, n int) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Server: the generated constructor wires the Redis consumer for the
	// spec's receive operations.
	got := make(chan *demo.LightCommand, n)
	srv, err := demo.NewServer(demoHandler{got: got},
		demo.WithAddr(addr),
		demo.WithGroup("demo-app"),
		demo.WithMode(mode),
	)
	if err != nil {
		return err
	}

	serverDone := make(chan error, 1)
	go func() { serverDone <- srv.Run(ctx) }()

	// Wait for the subscription/group to be established: publishing earlier
	// can lose messages in pubsub mode.
	select {
	case <-srv.Ready():
	case <-time.After(5 * time.Second):
		return fmt.Errorf("server did not become ready")
	}

	// Client: the generated constructor wires the Redis publisher.
	client, err := demo.NewClient(addr, demo.WithMode(mode))
	if err != nil {
		return err
	}

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
		case err := <-serverDone:
			return fmt.Errorf("server stopped: %w", err)
		}
	}

	stop()
	<-serverDone
	if err := client.Close(context.Background()); err != nil {
		return err
	}
	return srv.Close(context.Background())
}

// demoHandler implements the generated Handler interface.
type demoHandler struct {
	got chan *demo.LightCommand
}

// ReceiveLightCommand implements the receiveLightCommand operation.
func (h demoHandler) ReceiveLightCommand(_ context.Context, msg *demo.LightCommand) error {
	fmt.Printf("received command=%s sentAt=%s\n", msg.Payload.Command, msg.Payload.SentAt.Format(time.RFC3339))
	h.got <- msg
	return nil
}
