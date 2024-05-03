package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/gocarina/gocsv"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Constant fmt string formats for cache keys.
const (
	KeyCacheMessageDataFmt           = "cache:message:%s:roll"
	KeyCacheGuildNamedSettingFmt     = "cache:guild:%s:%s"
	KeyCacheInteractionTokenFmt      = "cache:token:%s"
	KeyCacheUserRecentFmt            = "cache:user:%s:recent"
	KeyCacheUserGlobalExpressionsFmt = "cache:user:%s::expressions"
	KeyCacheUserGuildExpressionsFmt  = "cache:user:%s:%s:expressions"

	KeyStateShardGuildsFmt = "state:shards:%s:guilds"
)

// Cache is an in-memory cache with a pass-through to the Redis backend.
type Cache struct {
	*lru.Cache[string, any]
	Redis *redis.Client
}

func NewCache(size int, redis *redis.Client) *Cache {
	c, err := lru.New[string, any](size)
	if err != nil {
		panic(err)
	}
	return &Cache{
		c,
		redis,
	}
}

// SMembers retrieves the members of a set with caching.
func (c *Cache) SMembers(ctx context.Context, k string) (smembers []string) {
	if v, ok := c.Get(k); ok {
		defer metrics.IncrCounter([]string{"cache", "hit"}, 1)
		smembers = v.([]string)
		return
	}
	defer metrics.IncrCounter([]string{"cache", "miss"}, 1)
	if c.Redis == nil {
		return
	}
	func() {
		defer metrics.MeasureSince([]string{"redis", "smembers"}, time.Now())
		smembers = c.Redis.SMembers(ctx, k).Val()
	}()
	defer c.Add(k, smembers)
	return
}

// SIsMember returns if a value is a member of a set within the cache.
func (c *Cache) SIsMember(ctx context.Context, k, v string) bool {
	smembers := c.SMembers(ctx, k)
	for _, m := range smembers {
		if m == v {
			return true
		}
	}
	return false
}

// ZRevRangeAll pulls the entire ranked set at a key in the cache in reverse
// order.
func (c *Cache) ZRevRangeAll(ctx context.Context, k string) (zrange []string) {
	if r, ok := c.Get(k); ok {
		defer metrics.IncrCounter([]string{"cache", "hit"}, 1)
		zrange = r.([]string)
		return
	}
	defer metrics.IncrCounter([]string{"cache", "miss"}, 1)
	if c.Redis == nil {
		return
	}
	func() {
		defer metrics.MeasureSince([]string{"redis", "zrevrange"}, time.Now())
		zrange = c.Redis.ZRevRange(ctx, k, 0, -1).Val()
	}()
	defer c.Add(k, zrange)
	return
}

func (c *Cache) HGetAll(ctx context.Context, k string) (hmap map[string]string) {
	if h, ok := c.Get(k); ok {
		defer metrics.IncrCounter([]string{"cache", "hit"}, 1)
		hmap = h.(map[string]string)
		return
	}
	defer metrics.IncrCounter([]string{"cache", "miss"}, 1)
	if c.Redis == nil {
		return
	}
	func() {
		defer metrics.MeasureSince([]string{"redis", "hgetall"}, time.Now())
		hmap = c.Redis.HGetAll(ctx, k).Val()
	}()
	defer c.Add(k, hmap)
	return
}

var ErrNoRedisClient = errors.New("no Redis client")

// CacheRoll adds a roll to a user's cache of recent rolls. This can be called
// defered.
func CacheRoll(u *discordgo.User, r *NamedRollInput) (err error) {
	ctx := context.TODO()
	if ok, err := r.okForAutocomplete(ctx); !ok {
		return err
	}

	// skip if user does not want rolls cached
	if UserHasPreference(u, SettingNoRecent) {
		return nil
	}

	keyRecent := fmt.Sprintf(KeyCacheUserRecentFmt, u.ID)
	keySaved := fmt.Sprintf(KeyCacheUserGlobalExpressionsFmt, u.ID)

	_, err = DiceGolem.Cache.Redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		now := time.Now()
		pipe.ZAdd(ctx, keyRecent, redis.Z{
			Score:  float64(now.UnixMilli()),
			Member: r.Serialize(),
		})

		// purge outdated value from cache
		defer DiceGolem.Cache.Remove(keyRecent)

		// trim history. Firstly, trim set to maximum history using index
		// offset, then remove any entries older than "recent" date.
		pipe.ZRemRangeByRank(ctx, keyRecent, 0, int64(-1-DiceGolem.MaxHistory))
		pipe.ZRemRangeByScore(ctx, keyRecent, "-inf", fmt.Sprint(now.Add(-DiceGolem.RecentTTL).Unix()))

		// re-set TTLs
		pipe.Expire(ctx, keyRecent, DiceGolem.HistoryTTL)
		pipe.Expire(ctx, keySaved, DiceGolem.DataTTL)
		return nil
	})

	if err != nil {
		logger.Error("error caching roll", zap.Error(err))
	}
	return
}

// CachedSerials returns the cached serials of recent rolls by the user.
func CachedSerials(u *discordgo.User) (serials []string, err error) {
	ctx := context.TODO()
	key := fmt.Sprintf(KeyCacheUserRecentFmt, u.ID)
	// get roll list sorted from recent to earliest
	serials = DiceGolem.Cache.ZRevRangeAll(ctx, key)
	return
}

// CachedRolls returns the cached rolls for a user. If err is non-nil rolls will
// be an empty slice.
func CachedRolls(u *discordgo.User) ([]NamedRollInput, error) {
	defer metrics.MeasureSince([]string{"cache", "cached_rolls"}, time.Now())

	serials, err := CachedSerials(u)
	if err != nil {
		return []NamedRollInput{}, err
	}

	rolls := make([]NamedRollInput, len(serials))
	for i, serial := range serials {
		roll := NamedRollInput{}
		roll.Deserialize(serial)
		rolls[i] = roll
	}
	return rolls, err
}

func SavedNamedRolls(key string) RollSlice {
	ctx := context.TODO()
	hmap := DiceGolem.Cache.HGetAll(ctx, key)
	rolls := make([]*NamedRollInput, 0)
	for _, serial := range hmap {
		roll := new(NamedRollInput)
		bErr := json.Unmarshal([]byte(serial), roll)
		if bErr == nil {
			rolls = append(rolls, roll)
		} else {
			logger.Error("json unmarshal error", zap.Any("serial", serial))
		}
	}

	return rolls
}

// SavedSerials returns the serials of saved rolls by the user. If err
// is non-nil serials will be an empty slice.
func SavedNamedRollSerials(key string) ([]string, error) {
	rolls := SavedNamedRolls(key)
	logger.Debug("saved rolls", zap.Any("rolls", rolls))
	serials := make([]string, len(rolls))
	for i, roll := range rolls {
		serials[i] = roll.Serialize()
	}
	return serials, nil
}

func ExportExpressions(ctx context.Context, expressions RollSlice) *discordgo.File {
	out, err := gocsv.MarshalBytes(&expressions)
	if err != nil {
		return nil
	}
	return &discordgo.File{
		Name:        "expressions.csv",
		ContentType: "text/csv; charset=utf-8",
		Reader:      bytes.NewReader(out),
	}
}

func ImportExpressionsInteraction(ctx context.Context, data map[string]interface{}) error {
	s, i, _ := FromContext(ctx)
	// make sure there was data
	csvStr, ok := data["csv"].(string)
	if csvStr == "" || !ok {
		return MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("CSV data was empty! No changes will be made."))
	}
	// unmarshal CSV to list of rolls
	csv := []byte(csvStr)
	var rolls RollSlice
	if err := gocsv.UnmarshalBytes(csv, &rolls); err != nil {
		return MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Error reading CSV: "+err.Error()))
	}

	logger.Debug("unmarshaled data", zap.Any("rolls", rolls))
	if len(rolls) > DiceGolem.MaxExpressions {
		return MeasureInteractionRespond(s.InteractionRespond, i,
			newEphemeralResponse(fmt.Sprintf("Data contained more than the maximum of %d expressions to save.", DiceGolem.MaxExpressions)))
	}

	for n, roll := range rolls {
		roll.Clean()
		if err := roll.Validate(); err != nil {
			return MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse(fmt.Sprintf("Error validating expression %d: %v", n+1, err)))
		}
		if ok, err := roll.okForAutocomplete(ctx); !ok {
			return MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse(fmt.Sprintf("Cannot save expression %d: %v", n+1, err)))
		}
	}

	// all the rolls validated as best as they can be; replace what's in there
	key := fmt.Sprintf(KeyCacheUserGlobalExpressionsFmt, UserFromInteraction(i).ID)
	DiceGolem.Cache.Redis.Del(ctx, key)
	for _, roll := range rolls {
		SetNamedRoll(UserFromInteraction(i), i.GuildID, roll)
	}
	count := DiceGolem.Cache.Redis.HLen(ctx, key).Val()
	return MeasureInteractionRespond(s.InteractionRespond, i,
		newEphemeralResponse(fmt.Sprintf("Expressions saved! Total expressions: %d", count)))
}
