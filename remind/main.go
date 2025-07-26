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

	GoogleCredentials   string `env:"GOOGLE_CREDENTIALS,required"`
	GoogleSpreadsheetID string `env:"GOOGLE_SPREADSHEET_ID,required"`
}

type Events []Event

type Schedule struct {
	Date time.Time
	Events
}

func loadConfig(ctx context.Context) (*Config, error) {
	useSSM, err := strconv.ParseBool(os.Getenv("USE_SSM"))
	if err != nil {
		slog.Error("failed to parse USE_SSM", slog.Any("error", err))
		return nil, err
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
			return nil, err
		}
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		slog.Error("failed to parse environment variables", slog.Any("error", err))
		return nil, err
	}

	return &cfg, nil
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

	// 設定を読み込む
	cfg, err := loadConfig(ctx)
	if err != nil {
		slog.Error("failed to load config", slog.Any("error", err))
		return err
	}

	// 対象とする日付情報を作成する
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		slog.Warn("failed to load JST location, using fixed offset", "err", err)
		jst = time.FixedZone("JST", 9*60*60)
	}
	now := time.Now().In(jst)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jst)
	dates := []time.Time{
		today,
		today.AddDate(0, 0, 1), // 実行日の翌日
	}

	// イベント情報を取得するクライアントを作成する
	srv, err := NewSheetsService(ctx, []byte(cfg.GoogleCredentials))
	if err != nil {
		slog.Error("failed to init Google Sheets service", slog.Any("error", err))
		return err
	}
	r := &GoogleSheetReader{Service: srv}
	src := NewSheetSource(r, cfg)
	c := NewClient(src)

	// イベント情報を取得する
	var schedules []Schedule
	for _, d := range dates {
		events, err := c.Do(ctx, d)
		if err != nil {
			slog.Error("failed to get events", slog.Any("error", err))
			continue
		}
		schedules = append(schedules, Schedule{Date: d, Events: events})
	}

	// イベント情報を Discord チャンネルに投稿する
	if err := postScheduleToDiscord(cfg, schedules); err != nil {
		slog.Error("failed to post events to Discord", slog.Any("error", err))
		return err
	}

	return nil
}

func main() {
	lambda.Start(handleRequest)
}
