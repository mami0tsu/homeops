package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type DateFilterCondition struct {
	OnOrBefore string `json:"on_or_before,omitempty"`
	After      string `json:"after,omitempty"`
	IsEmpty    *bool  `json:"is_empty,omitempty"`
}

type NotionFilterClause struct {
	Property string               `json:"property,omitempty"`
	Date     *DateFilterCondition `json:"date,omitempty"`
	Or       []NotionFilterClause `json:"or,omitempty"`
}

type NotionFilter struct {
	And []NotionFilterClause `json:"and"`
}

type NotionQueryRequest struct {
	Filter NotionFilter `json:"filter"`
}

func ptrBool(b bool) *bool {
	return &b
}

func fetchEvents(ctx context.Context, cfg Config, t time.Time) (Events, error) {
	url := fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", cfg.NotionDatabaseID)
	date := t.Format("2006-01-02")

	filter := NotionQueryRequest{
		Filter: NotionFilter{
			And: []NotionFilterClause{
				{
					Property: "Start",
					Date: &DateFilterCondition{
						OnOrBefore: date,
					},
				},
				{
					Or: []NotionFilterClause{
						{
							Property: "End",
							Date: &DateFilterCondition{
								IsEmpty: ptrBool(true),
							},
						},
						{
							Property: "End",
							Date: &DateFilterCondition{
								After: date,
							},
						},
					},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(filter)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+cfg.NotionAPIKey)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch pages: %s", string(body))
	}

	var notionResp NotionResponse
	if err := json.NewDecoder(resp.Body).Decode(&notionResp); err != nil {
		return nil, err
	}

	events := getEvents(notionResp.Results, t)

	return events, nil
}
