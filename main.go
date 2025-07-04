package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/mitchellh/mapstructure"
	"github.com/travis-g/dice"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TODO: move into Bot
var (
	logger *zap.Logger
)

// ResponsePrefix is the prefix for every bot response. It is a zero-width space
// by default help reduce the chance of accidentally activating another bot.
var ResponsePrefix, _ = strconv.Unquote("\u200B")

// MaxResponseLength Discord message length.
const MaxResponseLength = 2000

var (
	manyDice = regexp.MustCompile(`(?i)(?:^|\b(?P<num>\d{3,}))(d|f)`)
)

// Discord library init
func init() {
	discordgo.APIVersion = "10"

	// logging init
	discordgo.Logger = func(msgL, caller int, format string, a ...any) {
		var f func(msg string, fields ...zapcore.Field)
		pc, _, _, _ := runtime.Caller(caller + 1)
		name := runtime.FuncForPC(pc).Name()
		switch msgL {
		case discordgo.LogDebug:
			f = logger.Debug
		case discordgo.LogInformational:
			f = logger.Info
		case discordgo.LogWarning:
			f = logger.Warn
		case discordgo.LogError:
			f = logger.Error
		}
		f(fmt.Sprintf(format, a...), zap.String("source", name))
	}
}

func main() {
	var startTime = time.Now()
	ctx := context.Background()

	DiceGolem = NewBotFromConfig(NewBotConfig())
	DiceGolem.Setup()
	logger.Debug("loaded config", zap.Any("config", DiceGolem))

	// open HTTP server for heap debugging
	go func() {
		_ = http.ListenAndServe(":6060", nil)
	}()

	if err := DiceGolem.Open(ctx); err != nil {
		logger.Fatal("error opening connections", zap.Error(err))
	}
	defer DiceGolem.Close()

	startDuration := time.Since(startTime)
	logger.Info("bot started", zap.Duration("duration", startDuration.Round(time.Millisecond)))

	go DiceGolem.EmitNotificationMessage(ctx, &discordgo.MessageSend{
		Content: ResponsePrefix,
		Embeds: []*discordgo.MessageEmbed{
			{
				Description: fmt.Sprintf("Started %d shards (%s)!", len(DiceGolem.Sessions), startDuration.Round(time.Millisecond).String()),
				Footer:      makeEmbedFooter(),
			},
		},
		Flags: discordgo.MessageFlagsSuppressNotifications,
	})

	go func() {
		if err := DiceGolem.ConfigureCommands(ctx); err != nil {
			logger.Error("commands", zap.Error(err))
		}
		logger.Debug("commands", zap.Any("object", DiceGolem.Commands))
	}()

	// if DBL token is provided, set up the background server count updater.
	if DiceGolem.TopToken != nil {
		logger.Info("dbl enabled")
		go func() {
			for range time.Tick(1 * time.Hour) {
				logger.Info("posting dbl server count")
				postGuildCount(DiceGolem)
			}
		}()
	}

	// wait 10 seconds before starting metrics
	if DiceGolem.Metrics != nil {
		logger.Info("metrics enabled")
		go func() {
			for range time.Tick(10 * time.Second) {
				go metrics.IncrCounter([]string{"core", "healthy"}, 1)
				emitStats(DiceGolem)
			}
		}()
	}

	// Wait here until CTRL-C or other term signal is received.
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func HandlePanic(s *discordgo.Session, i any) {
	switch v := i.(type) {
	case *discordgo.Interaction:
		InteractionRecover(s, v)
	case *discordgo.Message:
		// FIXME: better message panic recovery
		if r := recover(); r != nil {
			logger.Error("recovering from panic",
				zap.String("message", v.ID),
				zap.String("panic", fmt.Sprintf("%v", r)))
		}
	default:
		if r := recover(); r != nil {
			logger.Error("recovered from unhandled panic",
				zap.String("panic", fmt.Sprintf("%v", r)))
		}
	}
}

// InteractionRecover is a panic catcher for Interactions, which will send back
// a friendly apology message to the user.
func InteractionRecover(s *discordgo.Session, i *discordgo.Interaction) {
	r := recover()
	// if nothing happened, skip
	if r == nil {
		return
	}

	logger.Error("recovering from panic",
		zap.String("interaction", i.ID),
		zap.String("panic", fmt.Sprintf("%v", r)))
	if err := MeasureInteractionRespond(s.InteractionRespond, i,
		newEphemeralResponse(ErrUnexpectedError.Error()),
	); err != nil {
		logger.Error("error sending response", zap.Error(err))
	}
}

// HandleReady handles a Discord READY event.
func HandleReady(s *discordgo.Session, e *discordgo.Ready) {
	logger.Info("ready received",
		zap.Int("shards", s.ShardCount),
		zap.Int("shard", s.ShardID),
	)
	metrics.IncrCounter([]string{"core", "ready"}, 1)
	if err := s.UpdateGameStatus(0, DiceGolem.Status); err != nil {
		logger.Error("update game status", zap.Error(err))
	}
}

// HandleResume handles a Discord RESUME event.
func HandleResume(s *discordgo.Session, e *discordgo.Resumed) {
	logger.Warn("resumed",
		zap.Int("shard", s.ShardID),
	)
	metrics.IncrCounter([]string{"core", "resume"}, 1)
}

func HandleConnect(s *discordgo.Session, e *discordgo.Connect) {
	logger.Warn("connected",
		zap.Int("shard", s.ShardID),
	)
	metrics.IncrCounter([]string{"core", "connect"}, 1)
}

func HandleDisconnect(s *discordgo.Session, e *discordgo.Disconnect) {
	logger.Warn("disconnected",
		zap.Int("shard", s.ShardID),
	)
	metrics.IncrCounter([]string{"core", "disconnect"}, 1)
}

func HandleGuildCreate(s *discordgo.Session, e *discordgo.GuildCreate) {
	ctx := context.TODO()
	metrics.IncrCounter([]string{"core", "guild_create"}, 1)
	logger.Debug("guild create",
		zap.Int("shard", s.ShardID),
		zap.String("id", e.ID))
	DiceGolem.Cache.Redis.SAdd(ctx, fmt.Sprintf(KeyStateShardGuildsFmt, strconv.Itoa(s.ShardID)), e.ID)
}

func HandleGuildDelete(s *discordgo.Session, e *discordgo.GuildDelete) {
	ctx := context.TODO()
	logger.Debug("guild delete",
		zap.Int("shard", s.ShardID),
		zap.String("id", e.ID),
		zap.Bool("unavailable", e.Unavailable))
	if !e.Unavailable {
		defer metrics.IncrCounter([]string{"core", "guild_delete"}, 1)
		DiceGolem.Cache.Redis.SRem(ctx, fmt.Sprintf(KeyStateShardGuildsFmt, strconv.Itoa(s.ShardID)), e.ID)
	}
}

// RouteInteractionCreate routes a Discord Interaction creation sent to the bot
// to the appropriate sub-routers ands handlers based on type.
func RouteInteractionCreate(s *discordgo.Session, ic *discordgo.InteractionCreate) {
	// measure bot-side handling time
	defer metrics.MeasureSince([]string{"core", "handle_interaction"}, time.Now())

	// measure time since message was registered within Discord
	sent, err := discordgo.SnowflakeTimestamp(ic.ID)
	if err == nil {
		defer metrics.MeasureSince([]string{"core", "round_trip"}, sent)
	}

	// FIXME: this duration needs to be lengthier, and cut back depending on
	// interaction type (ex. roll vs. session restart)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	i := ic.Interaction
	defer InteractionRecover(s, i)
	go func() {
		// log some metrics
		if err == nil {
			metrics.MeasureSince([]string{"core", "gateway_latency"}, sent)
		}
		metrics.IncrCounter([]string{"core", "interaction"}, 1)
		metrics.IncrCounter([]string{"interaction", i.Type.String()}, 1)
		metrics.IncrCounterWithLabels([]string{"client", "by_locale"}, 1, []metrics.Label{
			{Name: "locale", Value: string(i.Locale)},
		})
	}()

	logger.Debug("interaction type", zap.String("type", i.Type.String()))
	ctx = NewContext(ctx, s, i, nil)
	ctx = context.WithValue(ctx, dice.CtxKeyMaxRolls, int(float64(DiceGolem.MaxDice)*1.1))

	logger.Debug("interaction type", zap.String("type", i.Type.String()))
	switch i.Type {
	// CHAT_INPUT type
	case discordgo.InteractionApplicationCommand:
		data := i.ApplicationCommandData()
		go metrics.IncrCounter(append([]string{"command"}, getApplicationCommandPaths(data)...), 1)
		logger.Debug("interaction create",
			zap.String("name", data.Name),
			zap.Int("shard", s.ShardID),
			zap.Int("type", int(i.Type)),
			zap.Any("data", ic),
		)
		command := data.Name
		if handle, ok := handlers[command]; ok {
			handle(ctx)
		} else {
			// handler doesn't exist for command
			if err := MeasureInteractionRespond(s.InteractionRespond, i,
				newEphemeralResponse(ErrInvalidCommand.Error())); err != nil {
				logger.Error("error sending response", zap.Error(err))
			}
			return
		}

	// Message component clicks, ex. buttons
	case discordgo.InteractionMessageComponent:
		logger.Debug("component interaction create",
			zap.String("id", i.ID),
			zap.String("component", i.MessageComponentData().CustomID),
			zap.Int("shard", s.ShardID),
			zap.Int("type", int(i.Type)),
			zap.Any("data", ic),
		)
		id := i.MessageComponentData().CustomID
		// if button was a macro button strip off the macro_ prefix and use the
		// ID as the rest of the expression
		if strings.HasPrefix(id, "macro_") {
			roll := strings.TrimPrefix(id, "macro_")
			_, response, _ := NewRollMessageResponseFromString(ctx, roll)
			_, err := s.ChannelMessageSendComplex(i.ChannelID, response)
			if err != nil {
				MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse(ErrSendMessagePermissions.Error()))
			} else {
				// if we sent correctly, clear the pending button press
				_ = MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseDeferredMessageUpdate,
				})
			}
		} else if handle, ok := handlers[id]; ok {
			// if it was a generic action button, handle the press
			handle(ctx)
		} else {
			// handler doesn't exist for sent command
			if err := MeasureInteractionRespond(s.InteractionRespond, i,
				newEphemeralResponse(ErrInvalidCommand.Error()),
			); err != nil {
				logger.Error("error sending response", zap.Error(err))
			}
			return
		}

	// Autocomplete events with users' partial input data
	case discordgo.InteractionApplicationCommandAutocomplete:
		data := i.ApplicationCommandData()
		opt, param := getFocusedOption(data)
		if opt == nil {
			logger.Warn("unfocused autocomplete interaction", zap.Any("data", data))
			// TODO: better error
			return
		}
		defer metrics.MeasureSince([]string{"core", "autocomplete"}, time.Now())
		if suggest, ok := suggesters[param]; ok {
			logger.Debug("calling suggester", zap.Any("name", param))
			suggest(ctx)
		} else {
			panic("unhandled autocomplete parameter: " + param)
		}

	// Modal submissions
	case discordgo.InteractionModalSubmit:
		data := i.ModalSubmitData()
		logger.Debug("modal in", zap.Any("data", data))

		switch data.CustomID {
		case "modal_save":
			data := getModalTextInputComponents(data)
			roll := new(NamedRollInput)
			_ = mapstructure.Decode(data, roll)
			logger.Debug("modal data", zap.Any("data", roll))
			roll.Clean()
			if err := roll.Validate(); err != nil {
				MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Request invalid: "+err.Error()))
				return
			}
			if err := SetNamedRoll(UserFromInteraction(i), i.GuildID, roll); err != nil {
				logger.Error("error saving roll", zap.Error(err))
				MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Something unexpected errored! Please try again later."))
				return
			}
			if err := MeasureInteractionRespond(s.InteractionRespond, i,
				newEphemeralResponse(fmt.Sprintf("Saved `%v`!", roll)),
			); err != nil {
				logger.Error("error sending message", zap.Error(err))
			}
		case "modal_import":
			ImportExpressionsInteraction(ctx, getModalTextInputComponents(data))
			return

		default:
			MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Sorry! You submitted an unexpected modal. Please try again later."))
			return
		}
	default:
		panic("unhandled interaction type: " + i.Type.String())
	}
}

func HandleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	defer HandlePanic(s, m.Message)
	go func() {
		if sent, err := discordgo.SnowflakeTimestamp(m.ID); err == nil {
			metrics.MeasureSince([]string{"core", "gateway_latency"}, sent)
		}
		metrics.IncrCounter([]string{"core", "message_in"}, 1)
	}()
	logger.Debug("message_in", zap.Int("shard", s.ShardID), zap.Any("message", m))
	// no content means there's no roll text to process (or it's not meant for
	// the bot to see at all)
	if m.Content == "" {
		return
	}

	// skip if it was a bot's message (DG's response or other bot)
	user := UserFromMessage(m.Message)
	if user.Bot {
		return
	}

	// skip if message was a special message type, ex. a user join message
	if m.Type != discordgo.MessageTypeDefault {
		return
	}

	// short-circuit if not mentioned in a guild message
	if m.GuildID != "" && !SelfInUsers(m.Mentions) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	logger.Debug("handle message",
		zap.String("id", m.ID),
		zap.String("chan", m.ChannelID),
		zap.String("guild", m.GuildID),
		zap.Int("shard", s.ShardID),
	)
	go func() {
		metrics.IncrCounter([]string{"core", "interaction"}, 1)
		metrics.IncrCounter([]string{"interaction", "message"}, 1)
	}()

	ctx = NewContext(ctx, s, nil, m.Message)
	_, message, rollErr := NewMessageResponseFromMessage(ctx, m.Message)
	// if nothing to send, short-circuit
	if message == nil {
		return
	}

	// cache expression for response as well
	resMessage, _ := s.ChannelMessageSendComplex(m.ChannelID, message)

	// if roll had an error, schedule cleanup of the response
	if rollErr != nil && resMessage != nil {
		go func() {
			time.Sleep(10 * time.Second)
			if err := s.ChannelMessageDelete(resMessage.ChannelID, resMessage.ID); err != nil {
				logger.Error("message delete", zap.Error(err), zap.String("channel", resMessage.ChannelID))
			}
		}()
	}
}

// HandleRateLimit handles a possible rate limit by Discord.
func HandleRateLimit(s *discordgo.Session, e *discordgo.RateLimit) {
	logger.Warn("rate limited", zap.Any("event", e))
	metrics.IncrCounter([]string{"core", "rate_limit"}, 1)
}
