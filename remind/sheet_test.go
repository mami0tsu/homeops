package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/sheets/v4"
)

type MockSheetReader struct {
	MockResponse *sheets.ValueRange
	MockError    error
}

func (m *MockSheetReader) GetValues(ctx context.Context, spreadsheetID string, readRange string) (*sheets.ValueRange, error) {
	return m.MockResponse, m.MockError
}

var tz = time.FixedZone("JST", 9*60*60)

var testEvents = []Event{
	{Name: "Active", Start: time.Date(2025, 1, 1, 0, 0, 0, 0, tz), End: time.Date(2025, 1, 30, 0, 0, 0, 0, tz)},
	{Name: "On End", Start: time.Date(2025, 1, 1, 0, 0, 0, 0, tz), End: time.Date(2025, 1, 10, 0, 0, 0, 0, tz)},
	{Name: "On Start", Start: time.Date(2025, 1, 21, 0, 0, 0, 0, tz), End: time.Date(2025, 1, 30, 0, 0, 0, 0, tz)},
}

func eventsToValueRange(events []Event) *sheets.ValueRange {
	header := []string{"Name", "Interval", "StartDate", "EndDate"}
	headerRow := make([]interface{}, len(header))
	for i, h := range header {
		headerRow[i] = h
	}

	values := make([][]interface{}, 0, len(events)+1)
	values = append(values, headerRow)

	for _, e := range events {
		row := []interface{}{
			e.Name,
			e.Interval,
			e.Start.Format("2006/01/02"),
			e.End.Format("2006/01/02"),
		}
		values = append(values, row)
	}

	return &sheets.ValueRange{Values: values}
}

func TestFetch(t *testing.T) {
	cfg := &Config{
		GoogleSpreadsheetID: "dummy",
	}

	mockData := eventsToValueRange(testEvents)

	mockDataWithInvalidRow := eventsToValueRange(testEvents)
	mockDataWithInvalidRow.Values = append(mockDataWithInvalidRow.Values, []interface{}{"Invalid Event", "Daily", "2025-01-01", "not-a-date"})

	tests := []struct {
		name          string
		mockReader    *MockSheetReader
		targetTime    time.Time
		expectError   bool
		expectedNames []string
	}{
		{
			name: "正常系/期間内の日付が指定されている場合",
			mockReader: &MockSheetReader{
				MockResponse: mockData,
			},
			targetTime:    time.Date(2025, 1, 15, 0, 0, 0, 0, tz),
			expectError:   false,
			expectedNames: []string{"Active"},
		},
		{
			name: "正常系/どの期間にも含まれない日付が指定されている場合",
			mockReader: &MockSheetReader{
				MockResponse: mockData,
			},
			targetTime:    time.Date(2025, 1, 31, 0, 0, 0, 0, tz),
			expectError:   false,
			expectedNames: []string{},
		},
		{
			name: "正常系/不正な行が混在している場合",
			mockReader: &MockSheetReader{
				MockResponse: mockDataWithInvalidRow,
			},
			targetTime:    time.Date(2025, 1, 15, 0, 0, 0, 0, tz),
			expectError:   false,
			expectedNames: []string{"Active"},
		},
		{
			name: "正常系/ヘッダー列が欠けている場合",
			mockReader: &MockSheetReader{
				MockResponse: &sheets.ValueRange{
					Values: [][]interface{}{
						{"Name", "Interval", "StartDate"}, // "EndDate" が欠けている
					},
				},
			},
			expectError:   false,
			expectedNames: []string{},
		},
		{
			name: "正常系/APIがエラーを返した場合",
			mockReader: &MockSheetReader{
				MockError: fmt.Errorf("API permission denied"),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := assert.New(t)
			tr := require.New(t)

			src := NewSheetSource(tt.mockReader, cfg)
			filtered, err := src.Fetch(context.Background(), tt.targetTime)

			if tt.expectError {
				tr.Error(err, "Expected an error, but got none")
				return
			}
			tr.NoError(err, "Did not expect an error")

			var filteredNames []string
			for _, e := range filtered {
				filteredNames = append(filteredNames, e.Name)
			}

			ta.ElementsMatch(tt.expectedNames, filteredNames, "Filtered events should match expected names")
		})
	}
}

func TestParseRow(t *testing.T) {
	tz := time.FixedZone("JST", 9*60*60)
	cfg := &Config{
		GoogleSpreadsheetID: "dummy",
	}
	src := NewSheetSource(nil, cfg)

	tests := []struct {
		name        string
		row         []interface{}
		expectError bool
		expected    *Event
	}{
		{
			name:        "正常系/行が正常である場合",
			row:         []interface{}{"Valid Event", "Daily", "2025/01/01", "2025/01/02"},
			expectError: false,
			expected: &Event{
				Name:     "Valid Event",
				Interval: "Daily",
				Start:    time.Date(2025, 1, 1, 0, 0, 0, 0, tz),
				End:      time.Date(2025, 1, 2, 0, 0, 0, 0, tz),
			},
		},
		{
			name:        "異常系/列数が足りない場合",
			row:         []interface{}{"Invalid Event", "Daily", "2025-07-21"},
			expectError: true,
		},
		{
			name:        "異常系/開始日が不正な形式である場合",
			row:         []interface{}{"Invalid StartDate Event", "Daily", "not-a-date", "2025-01-02"},
			expectError: true,
		},
		{
			name:        "異常系/終了日が不正な形式である場合",
			row:         []interface{}{"Invalid EndDate Event", "Daily", "2025/01/01", "not-a-date"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := assert.New(t)
			event, err := src.parseRow(tt.row)

			if tt.expectError {
				ta.Error(err)
			} else {
				ta.NoError(err)
				ta.Equal(*tt.expected, event, "Parsed event should match expected value")
			}
		})
	}
}

func TestFilter(t *testing.T) {
	// テスト用のSheetSourceインスタンスを生成 (readerとcfgはFilterメソッドでは使われないのでnilでも可)
	src := NewSheetSource(nil, nil)

	tests := []struct {
		name          string
		inputEvents   []Event
		targetTime    time.Time
		expectedNames []string
	}{
		{
			name:          "正常系/期間内の日付が指定されている場合",
			inputEvents:   testEvents,
			targetTime:    time.Date(2025, 1, 15, 0, 0, 0, 0, tz),
			expectedNames: []string{"Active"},
		},
		{
			name:          "正常系/期間開始日と一致する日付が指定されている場合",
			inputEvents:   testEvents,
			targetTime:    time.Date(2025, 1, 21, 0, 0, 0, 0, tz),
			expectedNames: []string{"Active", "On Start"},
		},
		{
			name:          "正常系/期間終了日と一致する日付が指定されている場合",
			inputEvents:   testEvents,
			targetTime:    time.Date(2025, 1, 10, 0, 0, 0, 0, tz),
			expectedNames: []string{"Active", "On End"},
		},
		{
			name:          "正常系/どの期間にも含まれない日付が指定されている場合",
			inputEvents:   testEvents,
			targetTime:    time.Date(2025, 1, 31, 0, 0, 0, 0, tz),
			expectedNames: []string{},
		},
		{
			name:          "正常系/入力が空である場合",
			inputEvents:   []Event{},
			targetTime:    time.Date(2025, 1, 31, 0, 0, 0, 0, tz),
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := assert.New(t)
			filtered := src.filter(tt.inputEvents, tt.targetTime)

			var filteredNames []string
			for _, e := range filtered {
				filteredNames = append(filteredNames, e.Name)
			}

			ta.ElementsMatch(tt.expectedNames, filteredNames, "Filtered events should match expected names")
		})
	}
}
