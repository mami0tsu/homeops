package main

import (
	"context"
	"strings"
	"time"
)

type Event struct {
	Name      string
	Interval  string    // e.g. Onetime, Weekly, Monthly, Yearly
	StartDate time.Time // e.g. 2025/01/01
	EndDate   time.Time // e.g. 2025/12/31
}

type EventSource interface {
	Fetch(ctx context.Context, t time.Time) ([]Event, error)
}

func (e *Event) isContain(t time.Time) bool {
	// t < e.Start もしくは e.End < t なら除外する
	if t.Before(e.StartDate) || t.After(e.EndDate) {
		return false
	}

	return true
}

func (e *Event) isMatch(t time.Time) bool {
	switch strings.ToLower(e.Interval) {
	case "oneshot":
		return t.Equal(e.StartDate)
	case "weekly":
		return t.Weekday() == e.StartDate.Weekday()
	case "monthly":
		return t.Day() == e.StartDate.Day()
	case "yearly":
		return t.Month() == e.StartDate.Month() && t.Day() == e.StartDate.Day()
	default:
		return false
	}
}
