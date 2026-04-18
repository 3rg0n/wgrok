package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/3rg0n/wgrok/go"
	"github.com/joho/godotenv"
)

func main() {
	cardFile := flag.String("card", "", "path to adaptive card JSON file")
	flag.Parse()

	_ = godotenv.Load(".env.sender")

	config, err := wgrok.SenderConfigFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	payload := strings.Join(flag.Args(), " ")
	if payload == "" {
		fmt.Fprintln(os.Stderr, "Usage: sender [--card card.json] <payload>")
		os.Exit(1)
	}

	sender := wgrok.NewSender(config)

	var card interface{}
	if *cardFile != "" {
		data, err := os.ReadFile(*cardFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Read card file: %v\n", err)
			os.Exit(1)
		}
		if err := json.Unmarshal(data, &card); err != nil {
			fmt.Fprintf(os.Stderr, "Parse card JSON: %v\n", err)
			os.Exit(1)
		}
	}

	result, err := sender.Send(payload, card)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Send error: %v\n", err)
		os.Exit(1)
	}

	if result.MessageID != "" {
		fmt.Fprintf(os.Stderr, "Message ID: %s\n", result.MessageID)
	}
	out, _ := json.MarshalIndent(result.PlatformResponse, "", "  ")
	fmt.Println(string(out))
}
