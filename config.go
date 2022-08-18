package main

import (
	"context"
	"time"

	"github.com/sethvargo/go-envconfig"
)

// Config is the core set of configuration parameters for the bot. The
// parameters can be read from environment variables.
type Config struct {
	APIToken   string        `env:"API_TOKEN,required"`
	APITimeout time.Duration `env:"API_TIMEOUT,default=3s"`
	Wait       time.Duration `env:"WAIT,default=20s"`
	Shard      int           `env:"SHARD,default=-1"`
	Count      int           `env:"COUNT,default=0"`
	Status     string        `env:"STATUS,default=with fate!"`
	State      bool          `env:"STATE,default=true"`

	// Owner(s) and the home server(s) where the bot should allow admin
	// commands and experimental features.
	Owners   []string `env:"OWNER,required"`
	Homes    []string `env:"HOME"`
	Channels []string `env:"CHANNEL"`

	// Addresses of Statsd metrics service (optional) and Redis cache.
	StatsdAddr string `env:"STATSD_ADDR,default="`
	RedisAddr  string `env:"REDIS_ADDR,default=localhost:6379"`

	SelfID string `env:"ID,required"`
	Debug  bool   `env:"DEBUG,default=false"`
	// Top.gg token
	TopToken string `env:"TOP_TOKEN"`

	// TTL levels, from internal caches to Redis cache
	CacheTTL   time.Duration `env:"CACHE,default=30m"`
	HistoryTTL time.Duration `env:"RECENT,default=72h"`

	// Number of recent rolls to keep in history
	MaxHistory int `env:"MAX_HISTORY,default=10"`

	// Max dice allowed to be rolled per request
	MaxDice int `env:"MAX_DICE,default=500"`
}

// BotConfig is a prefixed environment variable config.
type BotConfig struct {
	*Config `env:",prefix=GOLEM_"`
}

// Validate ensures bot configuration properties are valid.
func (c *Config) Validate() error {
	// TODO: implement
	return ErrNotImplemented
}

func NewBotConfig() *BotConfig {
	ctx := context.Background()

	var c BotConfig
	if err := envconfig.Process(ctx, &c); err != nil {
		panic(err)
	}
	if err := c.Validate(); err != nil {
		// TODO: error on validation error
	}
	return &c
}
