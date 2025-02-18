package main

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	green int = 0x3fb950
	gray  int = 0xcccccc
)

func postScheduleToDiscord(cfg Config, schedules []Schedule) error {
	var embeds []*discordgo.MessageEmbed
	for _, s := range schedules {
		embeds = append(embeds, createMessageEmbed(s))
	}

	dg, err := discordgo.New("Bot " + cfg.DiscordBotToken)
	if err != nil {
		return err
	}
	if err := dg.Open(); err != nil {
		return err
	}
	defer dg.Close()

	webhook, err := dg.WebhookCreate(cfg.DiscordChannelID, cfg.DiscordBotName, "")
	if err != nil {
		return err
	}

	_, err = dg.WebhookExecute(webhook.ID, webhook.Token, false, &discordgo.WebhookParams{
		Embeds: embeds,
	})
	if err != nil {
		return err
	}

	if err := dg.WebhookDelete(webhook.ID); err != nil {
		slog.Warn("failed to delete webhook", "error", err)
	}

	return nil
}

func createMessageEmbed(s Schedule) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:  fmt.Sprintf("%s (%s) のイベント", s.Date.Format("2006-01-02"), s.Date.Weekday().String()[:3]),
		Color:  getColorCode(s.Date),
		Fields: []*discordgo.MessageEmbedField{},
	}
	for _, e := range s.Events {
		field := &discordgo.MessageEmbedField{
			Name:   e.Name,
			Value:  fmt.Sprintf("Interval: %s", e.Interval),
			Inline: false,
		}
		embed.Fields = append(embed.Fields, field)
	}

	return embed
}

func getColorCode(t time.Time) int {
	if isToday(t) {
		return green
	}

	return gray
}

func isToday(t time.Time) bool {
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		slog.Error("failed to load JST location, using fixed offset", "err", err)
		jst = time.FixedZone("JST", 9*3600)
	}
	now := time.Now().In(jst)

	return t.Year() == now.Year() && t.Month() == now.Month() && t.Day() == now.Day()
}
