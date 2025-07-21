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

func TestFetch(t *testing.T) {
	tz := time.FixedZone("JST", 9*60*60)
	cfg := &Config{
		GoogleSpreadsheetID: "dummy",
	}

	tests := []struct {
		name          string
		mockReader    *MockSheetReader
		expectError   bool
		expectedCount int
		expectedFirst *Event
	}{
		{
			name: "正常系/取得したデータが正常である場合、全てのデータが返却される",
			mockReader: &MockSheetReader{
				MockResponse: &sheets.ValueRange{
					Values: [][]interface{}{
						{"Name", "Interval", "StartDate", "EndDate"},
						{"First Event", "Daily", "2025/01/01", "2025/01/02"},
						{"Second Event", "Weekly", "2025/01/01", "2025/01/02"},
					},
				},
			},
			expectError:   false,
			expectedCount: 2,
			expectedFirst: &Event{
				Name:     "First Event",
				Interval: "Daily",
				Start:    time.Date(2025, 1, 1, 0, 0, 0, 0, tz),
				End:      time.Date(2025, 1, 2, 0, 0, 0, 0, tz),
			},
		},
		{
			name: "正常系/取得したデータに異常なデータが含まれている場合、不正なデータ以外が返却される",
			mockReader: &MockSheetReader{
				MockResponse: &sheets.ValueRange{
					Values: [][]interface{}{
						{"Name", "Interval", "StartDate", "EndDate"},
						{"Valid Event", "Daily", "2025/01/01", "2025/01/02"},
						{"Invalid StartDate Record", "Weekly", "not-a-date", "2025/01/02"},
					},
				},
			},
			expectError:   false,
			expectedCount: 1,
			expectedFirst: &Event{
				Name:     "Valid Event",
				Interval: "Daily",
				Start:    time.Date(2025, 1, 1, 0, 0, 0, 0, tz),
				End:      time.Date(2025, 1, 2, 0, 0, 0, 0, tz),
			},
		},
		{
			name: "正常系/データが存在しない場合、何も返却しない",
			mockReader: &MockSheetReader{
				MockResponse: &sheets.ValueRange{
					Values: [][]interface{}{
						{"Name", "Interval", "StartDate", "EndDate"},
					},
				},
			},
			expectError:   false,
			expectedCount: 0,
		},
		{
			name: "正常系/ヘッダー列が欠けている場合、何も返却しない",
			mockReader: &MockSheetReader{
				MockResponse: &sheets.ValueRange{
					Values: [][]interface{}{
						{"Name", "Interval", "StartDate"}, // "EndDate" が欠けている
					},
				},
			},
			expectError:   false,
			expectedCount: 0,
		},
		{
			name: "異常系/APIがエラーを返した場合、エラーになる",
			mockReader: &MockSheetReader{
				MockError: fmt.Errorf("API permission denied"),
			},
			expectError:   true,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := assert.New(t)
			tr := require.New(t)

			src := NewSheetSource(tt.mockReader, cfg)
			events, err := src.Fetch(context.Background())

			if tt.expectError {
				tr.Error(err, "Expected an error, but got none")
			} else {
				tr.NoError(err, "Did not expect an error")
			}

			ta.Len(events, tt.expectedCount, "Number of events should match expected count")

			if tt.expectedFirst != nil && len(events) > 0 {
				ta.Equal(*tt.expectedFirst, events[0], "The first event should match the expected value")
			}
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
			name:        "正常系/行が正常である場合、Event構造体が返却される",
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
			name:        "異常系/列数が足りない場合、エラーになる",
			row:         []interface{}{"Invalid Event", "Daily", "2025-07-21"},
			expectError: true,
		},
		{
			name:        "異常系/開始日が不正な形式である場合、エラーになる",
			row:         []interface{}{"Invalid StartDate Event", "Daily", "not-a-date", "2025-01-02"},
			expectError: true,
		},
		{
			name:        "異常系/終了日が不正な形式である場合、エラーになる",
			row:         []interface{}{"Invalid EndDate Event", "Daily", "2025/01/01", "not-a-date"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := assert.New(t)
			event, err := src.ParseRow(tt.row)

			if tt.expectError {
				ta.Error(err)
			} else {
				ta.NoError(err)
				ta.Equal(*tt.expected, event, "Parsed event should match expected value")
			}
		})
	}
}
