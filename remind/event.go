package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Interval int

const (
	onetime Interval = iota
	weekly
	monthly
	yearly
)

func (i Interval) String() string {
	switch i {
	case onetime:
		return "Onetime"
	case weekly:
		return "Weekly"
	case monthly:
		return "Monthly"
	case yearly:
		return "Yearly"
	default:
		return "Unknown"
	}
}

func parseInterval(s string) (Interval, error) {
	switch strings.ToLower(s) {
	case "onetime":
		return onetime, nil
	case "weekly":
		return weekly, nil
	case "monthly":
		return monthly, nil
	case "yearly":
		return yearly, nil
	default:
		return -1, fmt.Errorf("invalid interval: %s", s)
	}
}

type Event struct {
	Name      string
	Interval  Interval  // e.g. Onetime, Weekly, Monthly, Yearly
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
	switch e.Interval {
	case onetime:
		return t.Year() == e.StartDate.Year() && t.Month() == e.StartDate.Month() && t.Day() == e.StartDate.Day()
	case weekly:
		return t.Weekday() == e.StartDate.Weekday()
	case monthly:
		return t.Day() == e.StartDate.Day()
	case yearly:
		return t.Month() == e.StartDate.Month() && t.Day() == e.StartDate.Day()
	default:
		return false
	}
}
