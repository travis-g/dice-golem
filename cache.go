package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	CacheKeyMessageDataFormat      = "cache:message:%s:roll"
	CacheKeyGuildSettingFormat     = "cache:guild:%s:%s"
	CacheKeyInteractionTokenFormat = "cache:token:%s"
	CacheKeyUserRecentFormat       = "cache:user:%s:recent"
	CacheKeyUserRollsFormat        = "cache:user:%s:expressions"
)

// pray this is never used in a roll or label
var delim = '|'

type RollSlice []*RollInput

func (r *RollInput) Serialize() string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(r.Expression)
	if r.Label != "" {
		b.WriteRune(delim)
		b.WriteString(r.Label)
	}
	return b.String()
}

// Deserialize loads a roll input from a serialized string.
func (r *RollInput) Deserialize(serial string) {
	if r == nil {
		r = new(RollInput)
	}
	if serial == "" {
		return
	}
	parts := strings.Split(serial, string(delim))
	r.Expression = parts[0]
	if len(parts) > 1 {
		r.Label = parts[1]
	}
}

var ErrNoRedisClient = errors.New("no Redis client")

// CacheRoll adds a roll to a user's cache of recent rolls.
func CacheRoll(u *discordgo.User, r *RollInput) (err error) {
	if DiceGolem.Redis == nil {
		return ErrNoRedisClient
	}
	// skip if user does not want rolls cached
	if HasPreference(u, SettingNoRecent) {
		return nil
	}
	key := fmt.Sprintf(CacheKeyUserRecentFormat, u.ID)

	_, err = DiceGolem.Redis.Pipelined(func(pipe *redis.Pipeline) error {
		now := time.Now()
		z := redis.Z{
			Score:  float64(now.UnixMilli()),
			Member: r.Serialize(),
		}
		pipe.ZAdd(key, z)
		// re-set TTL
		pipe.Expire(key, DiceGolem.HistoryTTL)

		// trim history. Firstly, trim set to maximum history using index
		// offset, then remove any entries older than "recent" date.
		pipe.ZRemRangeByRank(key, 0, int64(-1-DiceGolem.MaxHistory))
		pipe.ZRemRangeByScore(key, "-inf", fmt.Sprint(now.Add(-DiceGolem.RecentTTL).Unix()))
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
	key := fmt.Sprintf(CacheKeyUserRecentFormat, u.ID)
	// get roll list sorted from recent to earliest
	func() {
		defer metrics.MeasureSince([]string{"redis", "zrevrange"}, time.Now())
		slice := DiceGolem.Redis.ZRevRange(key, 0, -1)
		serials, err = slice.Result()
	}()
	return
}

func CachedRolls(u *discordgo.User) (rolls []RollInput, err error) {
	defer metrics.MeasureSince([]string{"cache", "cached_rolls"}, time.Now())
	if DiceGolem.Redis == nil {
		return []RollInput{}, ErrNoRedisClient
	}

	serials, err := CachedSerials(u)
	if err != nil {
		return []RollInput{}, err
	}
	for _, serial := range serials {
		roll := RollInput{}
		roll.Deserialize(serial)
		rolls = append(rolls, roll)
	}
	return rolls, err
}

func CachedNamedRolls(key string) []*NamedRollInput {
	cmd := DiceGolem.Redis.HGetAllMap(key)
	data, err := cmd.Result()
	if err != nil {
		return nil
	}
	rolls := []*NamedRollInput{}
	for _, serial := range data {
		roll := new(NamedRollInput)
		jErr := json.Unmarshal([]byte(serial), roll)
		if jErr == nil {
			rolls = append(rolls, roll)
		} else {
			logger.Error("json unmarshal error", zap.Any("serial", serial))
		}
	}

	return rolls
}

func ExportExpressions(expressions []*NamedRollInput) *discordgo.File {
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
