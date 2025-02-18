package main

import (
	"log/slog"
	"strings"
	"time"
)

type NotionResponse struct {
	Results []NotionPage `json:"results"`
}

type NotionPage struct {
	Properties map[string]NotionProperty `json:"properties"`
}

type NotionProperty struct {
	Title  []NotionText  `json:"title,omitempty"`
	Date   *NotionDate   `json:"date,omitempty"`
	Select *NotionSelect `json:"select,omitempty"`
}

type NotionText struct {
	PlainText string `json:"plain_text"`
}

type NotionDate struct {
	Start string `json:"start"`
}

type NotionSelect struct {
	Name string `json:"name"`
}

func getEvents(pages []NotionPage, t time.Time) Events {
	var events Events
	for _, p := range pages {
		event, err := getEvent(p)
		if err != nil {
			slog.Warn("failed to convert page", slog.Any("error", err))
			continue
		}

		if validateEvent(event, t) {
			events = append(events, event)
		}
	}

	return events
}

func getEvent(page NotionPage) (Event, error) {
	var event Event
	if prop, ok := page.Properties["Name"]; ok && len(prop.Title) > 0 {
		event.Name = prop.Title[0].PlainText
	}

	if prop, ok := page.Properties["Interval"]; ok && prop.Select != nil {
		event.Interval = prop.Select.Name
	}

	if prop, ok := page.Properties["Start"]; ok && prop.Date != nil {
		t, err := time.Parse("2006-01-02", prop.Date.Start)
		if err != nil {
			return Event{}, err
		}
		event.Start = t
	}

	if prop, ok := page.Properties["End"]; ok && prop.Date != nil {
		t, err := time.Parse("2006-01-02", prop.Date.Start)
		if err != nil {
			slog.Info("failed to parse end date", "error", err)
		} else {
			event.End = t
		}
	}

	return event, nil
}

func sameDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func validateEvent(event Event, t time.Time) bool {
	if !event.End.IsZero() {
		if t.Before(event.Start) || !t.Before(event.End) {
			return false
		}
	}

	switch strings.ToLower(event.Interval) {
	case "oneshot":
		return sameDate(t, event.Start)
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
