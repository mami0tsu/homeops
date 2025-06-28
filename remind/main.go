package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"log/slog"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/caarlos0/env/v11"
	"github.com/handlename/ssmwrap/v2"
)

type Config struct {
	NotionAPIKey     string `env:"NOTION_API_KEY,required"`
	NotionDatabaseID string `env:"NOTION_DATABASE_ID,required"`

	DiscordBotName   string `env:"DISCORD_BOT_NAME,required"`
	DiscordBotToken  string `env:"DISCORD_BOT_TOKEN,required"`
	DiscordChannelID string `env:"DISCORD_CHANNEL_ID,required"`
}

type Event struct {
	Name     string
	Interval string    // e.g. Oneshot, Weekly, Monthly, Yearly
	Start    time.Time // e.g. 2025-01-01
	End      time.Time // e.g. 2025-12-31
}

type Events []Event

type Schedule struct {
	Date time.Time
	Events
}

func loadConfig(ctx context.Context) (Config, error) {
	useSSM, err := strconv.ParseBool(os.Getenv("USE_SSM"))
	if err != nil {
		slog.Error("failed to parse USE_SSM", slog.Any("error", err))
		return Config{}, err
	}

	if useSSM {
		appEnv := os.Getenv("APP_ENV")
		rules := []ssmwrap.ExportRule{
			{
				Path:   fmt.Sprintf("/%s/remind/discord/*", appEnv),
				Prefix: "DISCORD_",
			},
			{
				Path:   fmt.Sprintf("/%s/remind/notion/*", appEnv),
				Prefix: "NOTION_",
			},
		}
		if err := ssmwrap.Export(ctx, rules, ssmwrap.ExportOptions{}); err != nil {
			slog.Error("failed to get parameters from SSM", slog.Any("error", err))
			return Config{}, err
		}
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		slog.Error("failed to parse environment variables", slog.Any("error", err))
		return Config{}, err
	}

	return cfg, nil
}

func NewLogger() *slog.Logger {
	opts := slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.MessageKey:
				return slog.Attr{Key: "message", Value: attr.Value}
			}
			return attr
		},
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &opts))

	return logger
}

func handleRequest(ctx context.Context) error {
	slog.SetDefault(NewLogger())

	cfg, err := loadConfig(ctx)
	if err != nil {
		slog.Error("failed to load config", slog.Any("error", err))
		return err
	}

	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		slog.Warn("failed to load JST location, using fixed offset", "err", err)
		jst = time.FixedZone("JST", 9*60*60)
	}
	nowJST := time.Now().In(jst)

	var schedules []Schedule
	for _, t := range []time.Time{nowJST, nowJST.Add(time.Hour * 24)} {
		events, err := fetchEvents(ctx, cfg, t)
		if err != nil {
			slog.Error("failed to fetch events from Notion", "error", err)
			return err
		}
		schedules = append(schedules, Schedule{Date: t, Events: events})
	}
	slog.Info("succeeded to fetch events from Notion")

	if err := postScheduleToDiscord(cfg, schedules); err != nil {
		slog.Error("failed to post events to Discord", slog.Any("error", err))
		return err
	}
	slog.Info("succeeded to post events to Discord")

	return nil
}

func main() {
	lambda.Start(handleRequest)
}
