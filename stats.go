package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/armon/go-metrics"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/bwmarrin/discordgo"
	"github.com/shirou/gopsutil/mem"
)

var humanfmt *message.Printer

func init() {
	humanfmt = message.NewPrinter(language.English)
}

var startTime time.Time

func init() {
	startTime = time.Now()
}

// TODO: require a context
func makeStatsEmbed() []*discordgo.MessageEmbed {
	guilds, largeGuilds, _, _ := guildCount(DiceGolem)

	rolls, err := DiceGolem.Redis.Get("rolls:total").Int64()
	if err != nil {
		logger.Warn("stats", zap.String("error", "can't retrieve roll count"))
		rolls = -1
	}

	return []*discordgo.MessageEmbed{
		{
			Timestamp: time.Now().Local().Format(time.RFC3339),
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Guilds",
					Value:  humanfmt.Sprintf("%d", guilds),
					Inline: true,
				},
				{
					Name:   "Large Guilds",
					Value:  humanfmt.Sprintf("%d", largeGuilds),
					Inline: true,
				},
				{
					Name:  "Rolls",
					Value: humanfmt.Sprintf("%d", rolls),
				},
			},
		},
	}
}

// TODO: require a context
func makeStateEmbed() []*discordgo.MessageEmbed {
	stateGuilds, _, statesShards, _ := guildCount(DiceGolem)

	memstats := runtime.MemStats{}
	runtime.ReadMemStats(&memstats)
	sysmem, _ := mem.VirtualMemoryWithContext(context.Background())

	return []*discordgo.MessageEmbed{
		{
			Timestamp: time.Now().Local().Format(time.RFC3339),
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Uptime",
					Value:  time.Since(startTime).Round(time.Second).String(),
					Inline: true,
				},
				{
					Name:   "Cache",
					Value:  humanfmt.Sprintf("%d", len(DiceGolem.Cache.Items())),
					Inline: true,
				},
				{
					Name: "Memory",
					Value: fmt.Sprintf("%s (%.2f%%)",
						humanize.Bytes(memstats.Alloc), 100.0*float64(memstats.Alloc)/float64(sysmem.Available),
					),
					Inline: true,
				},
				{
					Name:   "Guilds",
					Value:  humanfmt.Sprintf("%d", stateGuilds),
					Inline: true,
				},
				{
					Name:   "Shards",
					Value:  humanfmt.Sprintf("`%v`", statesShards),
					Inline: true,
				},
			},
		},
	}
}

// emitStats emits gauge metrics. It will try to emit as
// many metrics as possible.
func emitStats(b *Bot) {
	metrics.SetGauge([]string{"core", "heartbeat"}, float32(b.DefaultSession.HeartbeatLatency()/time.Millisecond))
	if DiceGolem.Redis == nil {
		return
	}
	rolls, err := DiceGolem.Redis.Get("rolls:total").Int64()
	if err != nil {
		logger.Warn("metrics", zap.String("error", "can't retrieve roll count"))
	}
	metrics.SetGauge([]string{"rolls", "total"}, float32(rolls))
}
