package main

import (
	"context"
	"time"
)

type Event struct {
	Name     string
	Interval string    // e.g. Onetime, Weekly, Monthly, Yearly
	Start    time.Time // e.g. 2025-01-01
	End      time.Time // e.g. 2025-12-31
}

type EventSource interface {
	Fetch(ctx context.Context) ([]Event, error)
}

// FIXME
// type Client struct {
// 	source EventSource
// }
//
// func NewClient(src EventSource) *Client {
// 	return &Client{source: src}
// }
//
// func (c *Client) Do(ctx context.Context) error {
// 	events, err := c.source.Fetch(ctx)
// 	if err != nil {
// 	}
// 	return nil
// }
