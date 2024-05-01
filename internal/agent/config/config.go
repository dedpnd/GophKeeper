// Package config gets settings from environment variables or command line arguments.
package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	env "github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

// ConfigENV contains app settings.
type ConfigENV struct {
	Command    string
	JWT        string `env:"JWT"`
	ServerAddr string `json:"server_addr" env:"SERVER_ADDR"`
}

// GetConfig get app settings.
func GetConfig() (*ConfigENV, error) {
	var eCfg ConfigENV
	configPath := "config/agent.json"

	flag.StringVar(&eCfg.Command, "c", "", "command for GophKeeper storage")
	flag.Parse()

	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&eCfg); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("failed close config file: %w", err)
	}

	err = godotenv.Load(".env")
	if err != nil {
		return nil, fmt.Errorf("failed load .env file: %w", err)
	}

	err = env.Parse(&eCfg)
	if err != nil {
		return nil, fmt.Errorf("failed parsing environment variables: %w", err)
	}

	return &eCfg, nil
}
