package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/3rg0n/wgrok/go"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env.receiver")

	config, err := wgrok.ReceiverConfigFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	handler := func(slug, payload string, cards []interface{}, fromSlug string) {
		fmt.Printf("[RECEIVED] slug=%s from=%s payload=%s\n", slug, fromSlug, payload)
		if len(cards) > 0 {
			for i, card := range cards {
				out, _ := json.MarshalIndent(card, "", "  ")
				fmt.Printf("[CARD %d]\n%s\n", i+1, string(out))
			}
		}
	}

	receiver := wgrok.NewReceiver(config, handler)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := receiver.Listen(ctx); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
