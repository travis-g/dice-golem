package main

import (
	"context"
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
	// DefaultSession is the first session created by the bot with Discord. Any
	// DMs must be sent using DefaultSession, which should always be a pointer
	// to Sessions[0].
	DefaultSession *discordgo.Session
	Sessions       []*discordgo.Session
	Commands       BotCommands
	Cache          *Cache
	Redis          *redis.Client
}

var DiceGolem *Bot

func NewBotFromConfig(c *BotConfig) (b *Bot) {
	b = &Bot{
		BotConfig: c,
	}
	b.Commands.Global = CommandsGlobalChat
	b.Commands.Home = CommandsHomeChat
	b.Cache = NewCache(b.CacheTTL, 5*time.Minute)
	return b
}

// intent is the intent value the bot uses when identifying to Discord.
const intent = discordgo.IntentsDirectMessages | discordgo.IntentsGuildMessages | discordgo.IntentGuilds

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
		b.Redis = redis.NewClient(&redis.Options{Addr: b.RedisAddr, DB: 0})
		_, err := b.Redis.Ping().Result()

		if err != nil {
			logger.Fatal("failed to connect to redis", zap.Error(err))
		}
		logger.Info("connected to redis", zap.String("address", b.RedisAddr))
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

	for i := range b.Sessions {
		s, err := discordgo.New("Bot " + b.APIToken)
		if err != nil {
			return err
		}
		s.ShardCount = shards
		s.ShardID = i
		// set the intent
		s.Identify.Intents = intent
		s.LogLevel = discordgo.LogInformational

		s.State.TrackEmojis = false
		s.State.TrackThreadMembers = false
		s.State.TrackVoice = false

		s.AddHandler(HandleReady)
		s.AddHandler(HandleResume)
		s.AddHandler(HandleGuildCreate)
		s.AddHandler(HandleRateLimit)
		s.AddHandler(RouteInteractionCreate)
		s.AddHandler(HandleMessageCreate)
		// TODO: handle edits and deletes

		b.Sessions[i] = s
	}

	// use session 0 as our default/DM session
	b.DefaultSession = b.Sessions[0]

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
		if DiceGolem.DefaultSession.State.User != nil || ctx.Err() != nil {
			logger.Info("user data", zap.Any("user", DiceGolem.DefaultSession.State.User))
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
	existingCommands, err := b.DefaultSession.ApplicationCommands(b.SelfID, "")
	if err != nil {
		logger.Error("error getting commands", zap.Error(err))
	}
	logger.Debug("command list", zap.Any("commands", existingCommands))

	// upload desired commands in bulk
	logger.Debug("bulk uploading commands")
	commands, err := b.DefaultSession.ApplicationCommandBulkOverwrite(b.SelfID, "", b.Commands.Global)
	if err != nil {
		logger.Error("error overwriting commands", zap.Error(err))
	}
	logger.Debug("configured commands", zap.Any("commands", commands))

	// configure home server Interactions using the default session
	for _, home := range b.Homes {
		_, err := b.DefaultSession.ApplicationCommandBulkOverwrite(b.SelfID, home, b.Commands.Home)
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
		err := s.Close()
		if err != nil {
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
		if _, err := b.DefaultSession.ChannelMessageSendComplex(channel, m); err != nil {
			logger.Error("error sending message", zap.String("channel", channel), zap.Error(err))
		}
	}
	return nil
}
