package main

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type SheetsClient struct {
	service *sheets.Service
	sheetID string
}

func NewSheetsClient(ctx context.Context, credentialsPath, sheetID string) (*SheetsClient, error) {
	srv, err := sheets.NewService(ctx, option.WithCredentialsFile(credentialsPath))
	if err != nil {
		return nil, fmt.Errorf("creating sheets service: %w", err)
	}
	return &SheetsClient{
		service: srv,
		sheetID: sheetID,
	}, nil
}

func (sc *SheetsClient) AppendRow(ctx context.Context, values []interface{}) error {
	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{values},
	}

	_, err := sc.service.Spreadsheets.Values.Append(sc.sheetID, "Testing!A:D", valueRange).
		ValueInputOption("USER_ENTERED").
		InsertDataOption("INSERT_ROWS").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("appending row to sheet: %w", err)
	}

	log.Printf("Appended row to Google Sheets: %v", values)
	return nil
}
