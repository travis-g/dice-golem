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
					Value:  fmt.Sprintf("%s\n<t:%d>", time.Since(startTime).Round(time.Second).String(), startTime.Unix()),
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
					Name:   "Shards",
					Value:  humanfmt.Sprintf("`%v`", statesShards),
					Inline: true,
				},
				{
					Name:   "Guilds",
					Value:  humanfmt.Sprintf("%d", stateGuilds),
					Inline: true,
				},
			},
		},
	}
}

// emitStats emits telemetry metrics. It will try to emit as
// many metrics as possible.
func emitStats(b *Bot) {
	metrics.SetGauge([]string{"core", "heartbeat"}, float32(b.DefaultSession.HeartbeatLatency()/time.Millisecond))
	metrics.SetGauge([]string{"core", "cache_size"}, float32(len(DiceGolem.Cache.Items())))
	guilds, _, _, err := guildCount(DiceGolem)
	if err == nil {
		metrics.SetGauge([]string{"guilds", "total"}, float32(guilds))
	}

	// redis cache metrics
	if DiceGolem.Redis == nil {
		return
	}
	go func() {
		defer metrics.MeasureSince([]string{"redis", "ping"}, time.Now())
		_ = DiceGolem.Redis.Ping()
	}()
	if rolls, err := DiceGolem.Redis.Get("rolls:total").Int64(); err == nil {
		metrics.SetGauge([]string{"rolls", "total"}, float32(rolls))
	} else {
		logger.Warn("metrics", zap.String("error", "can't retrieve roll count"))
	}
}
