package main

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/redis/go-redis/v9"
	"github.com/travis-g/dice"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Bot is a state wrapper of sharded session management.
type Bot struct {
	*BotConfig
	Sessions []*discordgo.Session
	Commands BotCommands
	Cache    *Cache
	Metrics  *metrics.Metrics

	// The bot user
	User *discordgo.User
}

var DiceGolem *Bot

func NewBotFromConfig(c *BotConfig) (b *Bot) {
	b = &Bot{
		BotConfig: c,
	}
	b.Commands.Global = CommandsGlobalChat
	b.Commands.Home = CommandsHomeChat
	return b
}

// intent is the ORed bit intent value the bot uses when identifying to Discord.
const intent = discordgo.IntentDirectMessages | discordgo.IntentGuildMessages | discordgo.IntentGuilds

func (b *Bot) Setup() {
	ctx := context.Background()
	var err error

	dice.MaxRolls = uint64(b.MaxDice)

	// Set up logging
	switch b.Debug {
	case true:
		logger, err = zap.NewDevelopment()
	default:
		cfg := zap.NewProductionConfig()
		cfg.OutputPaths = []string{
			"stdout",
			"dice-golem.log",
		}
		cfg.ErrorOutputPaths = []string{
			"stderr",
			"dice-golem.log",
		}
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		logger, err = cfg.Build()
	}
	if err != nil {
		panic(err)
	}

	// Set up metrics
	if b.StatsdAddr != nil {
		sink, err := metrics.NewStatsdSink(*b.StatsdAddr)
		if err != nil {
			logger.Error("statsd", zap.Error(err))
		}
		if b.Metrics, err = metrics.NewGlobal(metrics.DefaultConfig("dice-golem"), sink); err != nil {
			logger.Error("metrics", zap.Error(err))
		}
	}

	// HACK: support an in-mem only option
	if b.RedisAddr != "" {
		logger.Info("connecting to redis", zap.String("address", b.RedisAddr))

		redis := redis.NewClient(&redis.Options{Addr: b.RedisAddr, DB: 0})
		b.Cache = NewCache(b.CacheSize, redis)
		if _, err := b.Cache.Redis.Ping(ctx).Result(); err != nil {
			logger.Error("failed to connect to redis", zap.Error(err))
		}
	}
}

// Open opens sharded sessions based on Discord's /gateway/bot response and
// returns the number of shards spawned.
func (b *Bot) Open(ctx context.Context) error {
	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + b.APIToken)
	if err != nil {
		logger.Fatal("error creating Discord session", zap.Error(err))
	}

	dg.StateEnabled = false

	gr, err := dg.GatewayBot()
	if err != nil {
		logger.Fatal("error querying gateway", zap.Error(err))
	}
	logger.Info("gateway response", zap.Any("data", gr))

	shards := int(math.Max(float64(gr.Shards), float64(b.MinShards)))
	b.Sessions = make([]*discordgo.Session, shards)

	// clear stale state cache
	if DiceGolem.Cache.Redis != nil {
		if _, err := DiceGolem.Cache.Redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			keys := DiceGolem.Cache.Redis.Keys(ctx, fmt.Sprintf(KeyStateShardGuildsFmt, "*")).Val()
			for _, key := range keys {
				DiceGolem.Cache.Redis.Del(ctx, key)
			}
			return nil
		}); err != nil {
			logger.Error("error clearing Redis", zap.Error(err))
		}
	}

	for i := range b.Sessions {
		s, err := discordgo.New("Bot " + b.APIToken)
		if err != nil {
			return err
		}
		if !b.BotConfig.State {
			s.StateEnabled = false
		}
		s.ShardCount = shards
		s.ShardID = i

		// set the intent
		s.Identify.Intents = intent

		if b.Debug {
			s.LogLevel = discordgo.LogDebug
		} else {
			s.LogLevel = discordgo.LogInformational
		}

		s.State.TrackChannels = false
		s.State.TrackThreads = false
		s.State.TrackEmojis = false
		s.State.TrackMembers = false
		s.State.TrackThreadMembers = false
		s.State.TrackRoles = false
		s.State.TrackVoice = false
		s.State.TrackPresences = false

		s.AddHandler(HandleReady)
		s.AddHandler(HandleResume)
		s.AddHandler(HandleGuildCreate)
		s.AddHandler(HandleGuildDelete)
		s.AddHandler(HandleRateLimit)
		s.AddHandler(RouteInteractionCreate)
		s.AddHandler(HandleMessageCreate)
		// TODO: handle edits and deletes

		b.Sessions[i] = s
	}

	// open sessions with waiting using a worker pool of max_concurrency:
	// https://gobyexample.com/worker-pools
	workerFunc := func(id int, sessions <-chan *discordgo.Session, done chan<- error) {
		for j := range sessions {
			err := openSession(j)
			if err != nil {
				logger.Error("error opening session", zap.Int("shard", id), zap.Error(err))
			}
			done <- err
		}
	}

	sessions := make(chan *discordgo.Session, len(b.Sessions))
	done := make(chan error, len(b.Sessions))
	defer close(sessions)

	// start concurrent workers
	for w := 1; w <= gr.SessionStartLimit.MaxConcurrency; w++ {
		go workerFunc(w, sessions, done)
	}

	// forward each session to the work pool
	for _, s := range b.Sessions {
		sessions <- s
	}
	// block until bot is at least self-aware
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		DiceGolem.User, err = DiceGolem.Sessions[0].User(DiceGolem.SelfID)
		if err == nil || ctx.Err() != nil {
			logger.Info("user data", zap.Any("user", DiceGolem.User))
			break
		}
	}

	// wait until sessions are open
	for e := 0; e < len(b.Sessions); e++ {
		<-done
	}
	return nil
}

func openSession(s *discordgo.Session) (err error) {
	logger.Info("opening session", zap.Int("shard", s.ShardID))
	if err = s.Open(); err != nil {
		logger.Error("error opening session", zap.Error(err))
	} else {
		logger.Info("session opened", zap.Int("id", s.ShardID))
	}
	return err
}

func closeSession(s *discordgo.Session) (err error) {
	logger.Info("closing session", zap.Int("shard", s.ShardID))
	if err = s.Close(); err != nil {
		logger.Error("error closing session", zap.Error(err))
	} else {
		logger.Info("session closed", zap.Int("id", s.ShardID))
	}
	return err
}

func restartSession(s *discordgo.Session) (err error) {
	logger.Info("restarting session", zap.Int("shard", s.ShardID))
	if err = closeSession(s); err != nil {
		logger.Error("error restarting session", zap.Error(err))
		return
	}
	if err = openSession(s); err != nil {
		logger.Error("error restarting session", zap.Error(err))
		return
	}
	return
}

// ConfigureCommands uploads the bot's set of global and guild commands using
// the default session.
func (b *Bot) ConfigureCommands(ctx context.Context) error {
	// check available global Commands
	existingCommands, err := b.Sessions[0].ApplicationCommands(b.SelfID, "")
	if err != nil {
		logger.Error("error getting commands", zap.Error(err))
	}
	logger.Debug("command list", zap.Any("commands", existingCommands))

	// upload desired commands in bulk
	logger.Debug("bulk uploading commands")
	commands, err := b.Sessions[0].ApplicationCommandBulkOverwrite(b.SelfID, "", b.Commands.Global)
	if err != nil {
		logger.Error("error overwriting commands", zap.Error(err))
	}
	logger.Debug("configured commands", zap.Any("commands", commands))

	// configure home server Interactions using the default session
	for _, home := range b.Homes {
		_, err := b.Sessions[0].ApplicationCommandBulkOverwrite(b.SelfID, home, b.Commands.Home)
		if err != nil {
			logger.Error("error overwriting guild commands", zap.String("guild", home), zap.Error(err))
		} else {
			logger.Debug("uploaded guild commands", zap.String("guild", home))
		}
	}
	return nil
}

// Close closes all open sessions.
func (b *Bot) Close() {
	for _, s := range b.Sessions {
		logger.Info(fmt.Sprintf("closing session %d", s.ShardID))
		if err := closeSession(s); err != nil {
			logger.Error("error closing session", zap.Error(err))
		}
	}
}

// IsOwner returns whether a user is also a bot owner.
func (b *Bot) IsOwner(user *discordgo.User) bool {
	for _, owner := range b.Owners {
		if user.ID == owner {
			return true
		}
	}
	return false
}

// EmitNotificationMessage sends a supplied message to all configured bot
// message channels.
func (b *Bot) EmitNotificationMessage(ctx context.Context, m *discordgo.MessageSend) error {
	if b.Silent {
		m.Flags = discordgo.MessageFlagsSuppressNotifications
	}
	for _, channel := range b.Channels {
		if _, err := b.Sessions[0].ChannelMessageSendComplex(channel, m); err != nil {
			logger.Error("error sending message", zap.String("channel", channel), zap.Error(err))
		}
	}
	return nil
}
