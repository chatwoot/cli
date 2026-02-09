package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/chatwoot/chatwoot-cli/internal/config"
	"github.com/chatwoot/chatwoot-cli/internal/sdk"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if cfg == nil || !cfg.IsValid() {
		cfg, err = runSetup()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
			os.Exit(1)
		}
	}

	client := sdk.NewClient(cfg.BaseURL, cfg.APIKey, cfg.AccountID)

	// Default action: list conversations
	convos, err := client.Conversations().List(sdk.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching conversations: %v\n", err)
		os.Exit(1)
	}

	if len(convos.Data.Payload) == 0 {
		fmt.Println("No conversations found.")
		return
	}

	for _, c := range convos.Data.Payload {
		sender := ""
		if c.Meta.Sender != nil {
			sender = c.Meta.Sender.Name
		}
		fmt.Printf("#%d [%s] %s\n", c.ID, c.Status, sender)
	}
}

func runSetup() (*config.Config, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Chatwoot CLI Setup")
	fmt.Println("==================")

	fmt.Print("Base URL (e.g. https://app.chatwoot.com): ")
	baseURL, _ := reader.ReadString('\n')
	baseURL = strings.TrimSpace(baseURL)

	fmt.Print("API Key: ")
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)

	fmt.Print("Account ID: ")
	accountIDStr, _ := reader.ReadString('\n')
	accountIDStr = strings.TrimSpace(accountIDStr)

	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID: %w", err)
	}

	cfg := &config.Config{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		AccountID: accountID,
	}

	if !cfg.IsValid() {
		return nil, fmt.Errorf("all fields are required")
	}

	if err := config.Save(cfg); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("Config saved successfully.")
	return cfg, nil
}
