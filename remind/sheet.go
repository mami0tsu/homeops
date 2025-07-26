package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
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

	filtered := s.filter(events, t)

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

func (s *SheetSource) filter(events []Event, t time.Time) []Event {
	var filtered []Event

	for _, e := range events {
		// 対象となる日付 t が t < e.Start もしくは e.End < t の関係なら除外する
		if t.Before(e.Start) || t.After(e.End) {
			continue
		}
		// 対象となるイベント e が通知条件にマッチしないなら除外する
		if !s.isMatch(e, t) {
			continue
		}
		filtered = append(filtered, e)
	}

	return filtered
}

func (s *SheetSource) isMatch(event Event, t time.Time) bool {
	switch strings.ToLower(event.Interval) {
	case "oneshot":
		return t.Equal(event.Start)
	case "weekly":
		return t.Weekday() == event.Start.Weekday()
	case "monthly":
		return t.Day() == event.Start.Day()
	case "yearly":
		return t.Month() == event.Start.Month() && t.Day() == event.Start.Day()
	default:
		slog.Warn("failed to validate event")
		return false
	}
}
