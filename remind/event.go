package main

import (
	"context"
	"strings"
	"time"
)

type Event struct {
	Name     string
	Interval string    // e.g. Onetime, Weekly, Monthly, Yearly
	Start    time.Time // e.g. 2025/01/01
	End      time.Time // e.g. 2025/12/31
}

type EventSource interface {
	Fetch(ctx context.Context, t time.Time) ([]Event, error)
}

func (e *Event) isContain(t time.Time) bool {
	// t < e.Start もしくは e.End < t なら除外する
	if t.Before(e.Start) || t.After(e.End) {
		return false
	}

	return true
}

func (e *Event) isMatch(t time.Time) bool {
	switch strings.ToLower(e.Interval) {
	case "oneshot":
		return t.Equal(e.Start)
	case "weekly":
		return t.Weekday() == e.Start.Weekday()
	case "monthly":
		return t.Day() == e.Start.Day()
	case "yearly":
		return t.Month() == e.Start.Month() && t.Day() == e.Start.Day()
	default:
		return false
	}
}
