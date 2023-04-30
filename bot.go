package main

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	redis "gopkg.in/redis.v3"
)

// Bot is a state wrapper of sharded session management.
type Bot struct {
	*BotConfig
	Sessions []*discordgo.Session
	Commands BotCommands
	Redis    *redis.Client
	User     *discordgo.User
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
	var err error

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
	// TODO: move into Bot
	if b.StatsdAddr != "" {
		sink, err := metrics.NewStatsdSink(b.StatsdAddr)
		if err != nil {
			logger.Error("statsd", zap.Error(err))
		}
		metrics.NewGlobal(metrics.DefaultConfig("dice-golem"), sink)
	}

	if b.RedisAddr != "" {
		logger.Info("connecting to redis", zap.String("address", b.RedisAddr))

		b.Redis = redis.NewClient(&redis.Options{Addr: b.RedisAddr, DB: 0})
		if _, err := b.Redis.Ping().Result(); err != nil {
			logger.Error("failed to connect to redis", zap.Error(err))
		}
	}
}

// Open opens sharded sessions based on Discord's /gateway/bot response and
// returns the number of shards spawned.
func (b *Bot) Open() error {
	defer metrics.MeasureSince([]string{"bot", "open"}, time.Now())

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

	shards := int(math.Max(float64(gr.Shards), 2))
	logger.Info("gateway response", zap.Any("data", gr))
	b.Sessions = make([]*discordgo.Session, shards)

	// clear stale state cache
	_, err = DiceGolem.Redis.Pipelined(func(pipe *redis.Pipeline) error {
		keys := DiceGolem.Redis.Keys(fmt.Sprintf(KeyStateShardGuildFmt, "*")).Val()
		for _, key := range keys {
			DiceGolem.Redis.Del(key)
		}
		return nil
	})

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

	// create a WaitGroup to ensure that we can open sessions concurrently but
	// still wait until we've also got core bot info back from Discord. This
	// should use a worker pool for bucketed sharding with max_concurrency:
	// https://gobyexample.com/worker-pools
	var wg sync.WaitGroup

	for i, s := range b.Sessions {
		wg.Add(1)
		go func(index int, session *discordgo.Session) {
			defer wg.Done()
			logger.Info("opening session", zap.Int("shard", index))
			if err := openSession(index, session); err != nil {
				logger.Error("error opening session", zap.Int("shard", index), zap.Error(err))
			}
		}(i, s)
	}

	// block until bot is at least self-aware
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	wg.Add(1)
	for {
		DiceGolem.User, err = DiceGolem.Sessions[0].User(DiceGolem.SelfID)
		if err == nil || ctx.Err() != nil {
			logger.Info("user data", zap.Any("user", DiceGolem.User))
			wg.Done()
			break
		}
	}

	// wait until sessions are open
	wg.Wait()

	return nil
}

func openSession(i int, s *discordgo.Session) (err error) {
	defer metrics.MeasureSince([]string{"session", "open"}, time.Now())
	if err = s.Open(); err != nil {
		logger.Error("error opening session", zap.Error(err))
	} else {
		logger.Info("session open", zap.Int("id", s.ShardID))
	}
	return err
}

// ConfigureCommands uploads the bot's set of global and guild commands using
// the default session.
func (b *Bot) ConfigureCommands() error {
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
		if err := s.Close(); err != nil {
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
func (b *Bot) EmitNotificationMessage(m *discordgo.MessageSend) error {
	for _, channel := range b.Channels {
		if _, err := b.Sessions[0].ChannelMessageSendComplex(channel, m); err != nil {
			logger.Error("error sending message", zap.String("channel", channel), zap.Error(err))
		}
	}
	return nil
}
