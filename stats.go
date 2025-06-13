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

// process init time, set during init()
var initTime time.Time

func init() {
	initTime = time.Now()
}

func makeStatsEmbed(ctx context.Context) []*discordgo.MessageEmbed {
	guilds, _, _ := guildCount(DiceGolem)

	rolls, err := DiceGolem.Cache.Redis.Get(ctx, "rolls:total").Int64()
	if err != nil {
		logger.Warn("stats", zap.String("error", "can't retrieve roll count"))
		rolls = -1
	}

	var totalExpressions int64
	keys := DiceGolem.Cache.Redis.Keys(ctx, fmt.Sprintf(KeyCacheUserGlobalExpressionsFmt, "*")).Val()
	for _, key := range keys {
		totalExpressions += DiceGolem.Cache.Redis.HLen(ctx, key).Val()
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

func makeHealthEmbed(ctx context.Context) []*discordgo.MessageEmbed {
	stateGuilds, statesShards, _ := guildCount(DiceGolem)

	memstats := runtime.MemStats{}
	runtime.ReadMemStats(&memstats)
	sysmem, _ := mem.VirtualMemoryWithContext(ctx)

	cacheSize := DiceGolem.Cache.Len()

	return []*discordgo.MessageEmbed{
		{
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Uptime",
					Value:  fmt.Sprintf("<t:%d> (%s)", initTime.Unix(), time.Since(initTime).Round(time.Second).String()),
					Inline: true,
				},
				{
					Name:   "Memory",
					Value:  fmt.Sprintf("%s (%.2f%%)", humanize.Bytes(memstats.Alloc), 100.0*float64(memstats.Alloc)/float64(sysmem.Available)),
					Inline: true,
				},
				{
					Name:   "Cache Size",
					Value:  fmt.Sprintf("%d", cacheSize),
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
					Value: fmt.Sprintf("`%v`", statesShards),
				},
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: "Version " + Revision,
			},
		},
	}
}

// emitStats emits telemetry metrics. It will try to emit as
// many metrics as possible.
func emitStats(b *Bot) {
	ctx := context.Background()
	metrics.SetGauge([]string{"core", "heartbeat"}, float32(b.Sessions[0].HeartbeatLatency()/time.Millisecond))
	guilds, _, err := guildCount(DiceGolem)
	if err == nil {
		metrics.SetGauge([]string{"guilds", "total"}, float32(guilds))
	}

	// redis cache metrics
	if DiceGolem.Cache.Redis == nil {
		return
	}

	var totalExpressions int64
	expressionsKeys := DiceGolem.Cache.Redis.Keys(ctx, fmt.Sprintf(KeyCacheUserGlobalExpressionsFmt, "*")).Val()
	for _, key := range expressionsKeys {
		totalExpressions += DiceGolem.Cache.Redis.HLen(ctx, key).Val()
	}
	metrics.SetGauge([]string{"storage", "expressions", "user_count"}, float32(len(expressionsKeys)))
	metrics.SetGauge([]string{"storage", "expressions", "count"}, float32(totalExpressions))

	var totalCache int64
	cacheKeys := DiceGolem.Cache.Redis.Keys(ctx, fmt.Sprintf(KeyCacheUserRecentFmt, "*")).Val()
	for _, key := range cacheKeys {
		totalCache += DiceGolem.Cache.Redis.ZCard(ctx, key).Val()
	}
	metrics.SetGauge([]string{"storage", "recent", "user_count"}, float32(len(cacheKeys)))
	metrics.SetGauge([]string{"storage", "recent", "count"}, float32(totalCache))

	go func() {
		t := time.Now() // defers don't work properly in a goroutine
		_ = DiceGolem.Cache.Redis.Ping(ctx)
		metrics.MeasureSince([]string{"redis", "ping"}, t)
	}()

	if rolls, err := DiceGolem.Cache.Redis.Get(ctx, "rolls:total").Int64(); err == nil {
		metrics.SetGauge([]string{"rolls", "total"}, float32(rolls))
	} else {
		logger.Warn("metrics", zap.String("error", "can't retrieve roll count"))
	}
}
