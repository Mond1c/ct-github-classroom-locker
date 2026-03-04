package main

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	GitHubAppID            int64
	GitHubPrivateKeyPath   string
	GitHubWebhookSecret    string
	ReviewTeamSlug         string
	GoogleCredentialsPath  string
	GoogleSheetID          string
	ListenAddr             string
}

func LoadConfig() (*Config, error) {
	appIDStr := os.Getenv("GITHUB_APP_ID")
	if appIDStr == "" {
		return nil, fmt.Errorf("GITHUB_APP_ID is required")
	}
	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("GITHUB_APP_ID must be a number: %w", err)
	}

	privateKeyPath := os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")
	if privateKeyPath == "" {
		return nil, fmt.Errorf("GITHUB_APP_PRIVATE_KEY_PATH is required")
	}

	webhookSecret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	if webhookSecret == "" {
		return nil, fmt.Errorf("GITHUB_WEBHOOK_SECRET is required")
	}

	reviewTeamSlug := os.Getenv("REVIEW_TEAM_SLUG")
	if reviewTeamSlug == "" {
		reviewTeamSlug = "teachers"
	}

	googleCredentialsPath := os.Getenv("GOOGLE_SHEETS_CREDENTIALS_PATH")
	if googleCredentialsPath == "" {
		return nil, fmt.Errorf("GOOGLE_SHEETS_CREDENTIALS_PATH is required")
	}

	googleSheetID := os.Getenv("GOOGLE_SHEET_ID")
	if googleSheetID == "" {
		return nil, fmt.Errorf("GOOGLE_SHEET_ID is required")
	}

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	return &Config{
		GitHubAppID:           appID,
		GitHubPrivateKeyPath:  privateKeyPath,
		GitHubWebhookSecret:   webhookSecret,
		ReviewTeamSlug:        reviewTeamSlug,
		GoogleCredentialsPath: googleCredentialsPath,
		GoogleSheetID:         googleSheetID,
		ListenAddr:            listenAddr,
	}, nil
}
