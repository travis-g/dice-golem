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

// process start time, set during init()
var startTime time.Time

func init() {
	startTime = time.Now()
}

// TODO: require a context
func makeStatsEmbed() []*discordgo.MessageEmbed {
	guilds, _, _ := guildCount(DiceGolem)

	rolls, err := DiceGolem.Redis.Get("rolls:total").Int64()
	if err != nil {
		logger.Warn("stats", zap.String("error", "can't retrieve roll count"))
		rolls = -1
	}

	var totalExpressions int64
	keys := DiceGolem.Redis.Keys(fmt.Sprintf(KeyUserGlobalExpressionsFmt, "*")).Val()
	for _, key := range keys {
		totalExpressions += DiceGolem.Redis.HLen(key).Val()
	}

	return []*discordgo.MessageEmbed{
		{
			Timestamp: time.Now().Local().Format(time.RFC3339),
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:  "Rolls",
					Value: humanfmt.Sprintf("%d", rolls),
				},
				{
					Name:   "Guilds",
					Value:  humanfmt.Sprintf("%d", guilds),
					Inline: true,
				},
				{
					Name:   "Expressions",
					Value:  humanfmt.Sprintf("%d", totalExpressions),
					Inline: true,
				},
			},
		},
	}
}

// TODO: require a context
func makeHealthEmbed() []*discordgo.MessageEmbed {
	stateGuilds, statesShards, _ := guildCount(DiceGolem)

	memstats := runtime.MemStats{}
	runtime.ReadMemStats(&memstats)
	sysmem, _ := mem.VirtualMemoryWithContext(context.Background())

	return []*discordgo.MessageEmbed{
		{
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Uptime",
					Value:  fmt.Sprintf("<t:%d> (%s)", startTime.Unix(), time.Since(startTime).Round(time.Second).String()),
					Inline: true,
				},
				{
					Name:   "Memory",
					Value:  fmt.Sprintf("%s (%.2f%%)", humanize.Bytes(memstats.Alloc), 100.0*float64(memstats.Alloc)/float64(sysmem.Available)),
					Inline: true,
				},
				{
					Name:   "Goroutines",
					Value:  humanfmt.Sprintf("%d", runtime.NumGoroutine()),
					Inline: true,
				},
				{
					Name:   "Guilds",
					Value:  humanfmt.Sprintf("%d", stateGuilds),
					Inline: true,
				},
				{
					Name:  "Shards",
					Value: humanfmt.Sprintf("`%v`", statesShards),
				},
			},
		},
	}
}

// emitStats emits telemetry metrics. It will try to emit as
// many metrics as possible.
func emitStats(b *Bot) {
	defer metrics.MeasureSince([]string{"core", "metrics"}, time.Now())
	metrics.SetGauge([]string{"core", "heartbeat"}, float32(b.Sessions[0].HeartbeatLatency()/time.Millisecond))
	guilds, _, err := guildCount(DiceGolem)
	if err == nil {
		metrics.SetGauge([]string{"guilds", "total"}, float32(guilds))
	}

	// redis cache metrics
	if DiceGolem.Redis == nil {
		return
	}

	var totalExpressions int64
	expressionsKeys := DiceGolem.Redis.Keys(fmt.Sprintf(KeyUserGlobalExpressionsFmt, "*")).Val()
	for _, key := range expressionsKeys {
		totalExpressions += DiceGolem.Redis.HLen(key).Val()
	}
	metrics.SetGauge([]string{"storage", "expressions", "user_count"}, float32(len(expressionsKeys)))
	metrics.SetGauge([]string{"storage", "expressions", "count"}, float32(totalExpressions))

	var totalCache int64
	cacheKeys := DiceGolem.Redis.Keys(fmt.Sprintf(KeyUserRecentFmt, "*")).Val()
	for _, key := range cacheKeys {
		totalCache += DiceGolem.Redis.ZCard(key).Val()
	}
	metrics.SetGauge([]string{"storage", "recent", "user_count"}, float32(len(cacheKeys)))
	metrics.SetGauge([]string{"storage", "recent", "count"}, float32(totalCache))

	go func() {
		t := time.Now() // defers don't work properly in a goroutine
		_ = DiceGolem.Redis.Ping()
		metrics.MeasureSince([]string{"redis", "ping"}, t)
	}()

	if rolls, err := DiceGolem.Redis.Get("rolls:total").Int64(); err == nil {
		metrics.SetGauge([]string{"rolls", "total"}, float32(rolls))
	} else {
		logger.Warn("metrics", zap.String("error", "can't retrieve roll count"))
	}
}
