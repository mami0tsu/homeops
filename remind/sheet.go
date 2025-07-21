package main

import (
	"context"
	"fmt"
	"time"

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
func (s *SheetSource) Fetch(ctx context.Context) ([]Event, error) {
	resp, err := s.reader.GetValues(ctx, s.config.GoogleSpreadsheetID, "event!A:D")
	if err != nil {
		return nil, err
	}

	// シートにヘッダーしか存在していない場合は早期リターンする
	if len(resp.Values) < 2 {
		return []Event{}, nil
	}

	// header := resp.Values[0]
	dataRows := resp.Values[1:]

	var events []Event
	for _, r := range dataRows {
		e, err := s.ParseRow(r)
		if err != nil {
			// パースできない行はスキップする
			continue
		}
		events = append(events, e)
	}

	return events, nil
}

func (s *SheetSource) ParseRow(r []interface{}) (Event, error) {
	maxIdx := 3
	if len(r) <= maxIdx {
		// 列が想定より少ない場合は早期リターンする
		return Event{}, fmt.Errorf("row has insufficient columns")
	}

	tz := time.FixedZone("JST", 9*60*60)
	startDate, err := time.ParseInLocation("2006/01/02", fmt.Sprintf("%v", r[startDateIdx]), tz)
	if err != nil {
		return Event{}, fmt.Errorf("failed to parse start date '%v': %w", r[startDateIdx], err)
	}
	endDate, err := time.ParseInLocation("2006/01/02", fmt.Sprintf("%v", r[endDateIdx]), tz)
	if err != nil {
		return Event{}, fmt.Errorf("failed to parse end date '%v': %w", r[endDateIdx], err)
	}

	return Event{
		Name:     fmt.Sprintf("%v", r[nameIdx]),
		Interval: fmt.Sprintf("%v", r[intervalIdx]),
		Start:    startDate,
		End:      endDate,
	}, nil
}
