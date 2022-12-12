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
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	redis "gopkg.in/redis.v3"
)

type cacheInterface interface {
	Get(k string) (interface{}, bool)
	SetWithTTL(k string, x interface{}, d time.Duration)
	Delete(k string)
	Items() map[string]cache.Item
}

type Cache struct {
	cache *cache.Cache
	ttl   time.Duration
}

func NewCache(defaultExpiration, cleanupInterval time.Duration) *Cache {
	return &Cache{
		cache: cache.New(defaultExpiration, cleanupInterval),
		ttl:   defaultExpiration,
	}
}

var _ cacheInterface = (*Cache)(nil)

// Get returns and item from the cache or nil, and a bool indicating if the item
// was found.
func (c *Cache) Get(k string) (interface{}, bool) {
	defer metrics.MeasureSince([]string{"cache", "get"}, time.Now())
	return c.cache.Get(k)
}

// SetWithTTL sets an item in the cache and binds a TTL. A TTL of 0 will use the
// cache's DefaultExpiration. A ttl of -1 will disable expiry.
func (c *Cache) SetWithTTL(k string, x interface{}, ttl time.Duration) {
	defer metrics.MeasureSince([]string{"cache", "set"}, time.Now())
	c.cache.Set(k, x, ttl)
	logger.Debug("cache set", zap.String("key", k), zap.Any("data", x))
}

// Delete removes an item from the cache.
func (c *Cache) Delete(k string) {
	defer metrics.MeasureSince([]string{"cache", "delete"}, time.Now())
	logger.Debug("cache delete", zap.String("key", k))
	c.cache.Delete(k)
}

// Items returns a copy of all items in the cache.
func (c *Cache) Items() map[string]cache.Item {
	defer metrics.MeasureSince([]string{"cache", "list"}, time.Now())
	return c.cache.Items()
}

// Constant fmt string formats for cache keys.
const (
	KeyMessageDataFmt           = "cache:message:%s:roll"
	KeyGuildSettingFmt          = "cache:guild:%s:%s"
	KeyInteractionTokenFmt      = "cache:token:%s"
	KeyUserRecentFmt            = "cache:user:%s:recent"
	KeyUserGlobalExpressionsFmt = "cache:user:%s::expressions"
)

// pray this is never used in a roll or label
var delim = '|'

type RollSlice []*NamedRollInput

var ErrNoRedisClient = errors.New("no Redis client")

// CacheRoll adds a roll to a user's cache of recent rolls.
func CacheRoll(u *discordgo.User, r *NamedRollInput) (err error) {
	if ok, err := r.okForAutocomplete(); !ok {
		return err
	}

	if DiceGolem.Redis == nil {
		return ErrNoRedisClient
	}
	// skip if user does not want rolls cached
	if HasPreference(u, SettingNoRecent) {
		return nil
	}
	keyRecent := fmt.Sprintf(KeyUserRecentFmt, u.ID)
	keySaved := fmt.Sprintf(KeyUserGlobalExpressionsFmt, u.ID)

	_, err = DiceGolem.Redis.Pipelined(func(pipe *redis.Pipeline) error {
		now := time.Now()
		z := redis.Z{
			Score:  float64(now.UnixMilli()),
			Member: r.Serialize(),
		}
		pipe.ZAdd(keyRecent, z)

		// re-set TTLs
		pipe.Expire(keyRecent, DiceGolem.HistoryTTL)
		pipe.Expire(keySaved, DiceGolem.DataTTL)

		// trim history. Firstly, trim set to maximum history using index
		// offset, then remove any entries older than "recent" date.
		pipe.ZRemRangeByRank(keyRecent, 0, int64(-1-DiceGolem.MaxHistory))
		pipe.ZRemRangeByScore(keyRecent, "-inf", fmt.Sprint(now.Add(-DiceGolem.RecentTTL).Unix()))
		return nil
	})

	if err != nil {
		logger.Error("error caching roll", zap.Error(err))
	}
	return
}

// CachedSerials returns the cached serials of recent rolls by the user. If err
// is non-nil serials will be an empty slice.
func CachedSerials(u *discordgo.User) (serials []string, err error) {
	if DiceGolem.Redis == nil {
		return []string{}, ErrNoRedisClient
	}
	key := fmt.Sprintf(KeyUserRecentFmt, u.ID)
	// get roll list sorted from recent to earliest
	func() {
		defer metrics.MeasureSince([]string{"redis", "zrevrange"}, time.Now())
		slice := DiceGolem.Redis.ZRevRange(key, 0, -1)
		serials, err = slice.Result()
	}()
	return
}

func CachedRolls(u *discordgo.User) (rolls []NamedRollInput, err error) {
	defer metrics.MeasureSince([]string{"cache", "cached_rolls"}, time.Now())
	if DiceGolem.Redis == nil {
		return []NamedRollInput{}, ErrNoRedisClient
	}

	serials, err := CachedSerials(u)
	if err != nil {
		return []NamedRollInput{}, err
	}
	for _, serial := range serials {
		roll := NamedRollInput{}
		roll.Deserialize(serial)
		rolls = append(rolls, roll)
	}
	return rolls, err
}

func CachedRecentRolls(key string) []*NamedRollInput {
	var serials []string
	// get roll list sorted from recent to earliest
	func() {
		defer metrics.MeasureSince([]string{"redis", "zrevrange"}, time.Now())
		slice := DiceGolem.Redis.ZRevRange(key, 0, -1)
		serials, _ = slice.Result()
	}()
	rolls := []*NamedRollInput{}
	for _, serial := range serials {
		roll := new(NamedRollInput)
		bErr := json.Unmarshal([]byte(serial), roll)
		if bErr == nil {
			rolls = append(rolls, roll)
		} else {
			roll.Deserialize(serial)
			if roll.Expression != "" {
				rolls = append(rolls, roll)
			}
		}
	}

	return rolls
}

func SavedNamedRolls(key string) []*NamedRollInput {
	cmd := DiceGolem.Redis.HGetAllMap(key)
	data, err := cmd.Result()
	if err != nil {
		return nil
	}
	rolls := []*NamedRollInput{}
	for _, serial := range data {
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
	if DiceGolem.Redis == nil {
		return []string{}, ErrNoRedisClient
	}
	data := SavedNamedRolls(key)
	logger.Debug("saved rolls", zap.Any("data", data))
	serials := make([]string, 0)
	for _, roll := range data {
		serials = append(serials, roll.Serialize())
	}
	return serials, nil
}

func ExportExpressions(ctx context.Context, expressions []*NamedRollInput) *discordgo.File {
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
