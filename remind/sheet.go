package main

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	nameIdx      = 0
	intervalIdx  = 1
	startDateIdx = 2
	endDateIdx   = 3
)

type SheetDataReader interface {
	GetValues(ctx context.Context, spreadsheetID, readRange string) (*sheets.ValueRange, error)
}

func NewSheetsService(ctx context.Context, credentials []byte) (*sheets.Service, error) {
	cfg, err := google.JWTConfigFromJSON(credentials, sheets.SpreadsheetsReadonlyScope)
	if err != nil {
		return nil, err
	}
	c := cfg.Client(ctx)
	srv, err := sheets.NewService(ctx, option.WithHTTPClient(c))
	if err != nil {
		return nil, err
	}
	return srv, nil
}

type GoogleSheetReader struct {
	Service *sheets.Service
}

func (gsr *GoogleSheetReader) GetValues(ctx context.Context, spreadsheetID, readRange string) (*sheets.ValueRange, error) {
	return gsr.Service.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
}

type SheetSource struct {
	reader SheetDataReader
	config *Config
}

// スプレッドシート用のデータソース
func NewSheetSource(reader SheetDataReader, cfg *Config) *SheetSource {
	return &SheetSource{
		reader: reader,
		config: cfg,
	}
}

// スプレッドシートからデータを取得した上でパースして返却する
func (s *SheetSource) Fetch(ctx context.Context, t time.Time) ([]Event, error) {
	resp, err := s.reader.GetValues(ctx, s.config.GoogleSpreadsheetID, "reminder!A:D")
	if err != nil {
		return nil, err
	}

	// シートにヘッダーしか存在していない場合は早期リターンする
	if len(resp.Values) < 2 {
		return []Event{}, nil
	}

	dataRows := resp.Values[1:]

	var events []Event
	for _, r := range dataRows {
		e, err := s.parseRow(r)
		if err != nil {
			// パースできない行はスキップする
			continue
		}
		events = append(events, e)
	}

	var filtered []Event
	for _, e := range events {
		if e.isContain(t) && e.isMatch(t) {
			filtered = append(filtered, e)
		}
	}

	return filtered, nil
}

func (s *SheetSource) parseDate(r []interface{}, index int) (time.Time, error) {
	tz := time.FixedZone("JST", 9*60*60)
	now := time.Now().In(tz)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, tz)

	if len(r) <= index || fmt.Sprintf("%v", r[index]) == "" {
		return today, nil
	}

	dateStr := fmt.Sprintf("%v", r[index])
	t, err := time.ParseInLocation("2006/01/02", dateStr, tz)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse date from column")
	}

	return t, nil
}

func (s *SheetSource) parseValue(r []interface{}, index int) (string, error) {
	if len(r) <= index || fmt.Sprintf("%v", r[index]) == "" {
		return "", fmt.Errorf("failed to parse value from column")
	}

	return fmt.Sprintf("%v", r[index]), nil
}

func (s *SheetSource) parseRow(r []interface{}) (Event, error) {
	name, err := s.parseValue(r, nameIdx)
	if err != nil {
		return Event{}, err
	}

	interval, err := s.parseValue(r, intervalIdx)
	if err != nil {
		return Event{}, err
	}

	startDate, err := s.parseDate(r, startDateIdx)
	if err != nil {
		return Event{}, err
	}

	endDate, err := s.parseDate(r, endDateIdx)
	if err != nil {
		return Event{}, err
	}

	return Event{
		Name:     name,
		Interval: interval,
		Start:    startDate,
		End:      endDate,
	}, nil
}
