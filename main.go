package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/travis-g/dice"
	"github.com/travis-g/dice/math"
	"go.uber.org/zap"
)

// Default variable settings.
const (
	DefaultTimeout = time.Second * 3
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

// Response templates for dice roll message responses.
var (
	ResponseTemplate      = "{{if .Name}}{{.Name}} rolled{{end}}{{if .Expression}} `{{.Expression}}`{{end}}{{if .Label}} *{{.Label}}*{{end}}: `{{.Rolled}}` = **{{.Result}}**"
	ResponseErrorTemplate = "​{{.Name}}: {{.FriendlyError}}"
)

var (
	responseResultTemplateCompiled = template.Must(
		template.New("result").Parse(ResponsePrefix + ResponseTemplate),
	)
	responseErrorTemplateCompiled = template.Must(
		template.New("error").Parse(ResponsePrefix + ResponseErrorTemplate),
	)
)

var (
	manyDice = regexp.MustCompile(`(?:^|\b(?P<num>\d{3,}))(d|f)`)
)

// Response is a message response for dice roll responses.
type Response struct {
	*math.ExpressionResult
	// Name is who made the roll (optional)
	Name          string
	Rolled        string
	Result        string
	Expression    string
	Label         string
	FriendlyError error
}

func main() {
	discordgo.APIVersion = "10"

	DiceGolem = NewBotFromConfig(NewBotConfig())
	DiceGolem.Setup()
	logger.Debug("loaded config", zap.Any("config", DiceGolem))

	shards, err := DiceGolem.Open()
	if err != nil {
		logger.Fatal("error opening connections", zap.Error(err))
	}
	defer DiceGolem.Close()
	logger.Info("bot started")

	DiceGolem.EmitNotificationMessage(&discordgo.MessageSend{
		Content: ResponsePrefix,
		Embeds: []*discordgo.MessageEmbed{
			{
				Description: fmt.Sprintf("Started %d shards!", shards),
				Footer: &discordgo.MessageEmbedFooter{
					Text:    DiceGolem.DefaultSession.State.User.Username,
					IconURL: DiceGolem.DefaultSession.State.User.AvatarURL("64"),
				},
			},
		},
	})

	if err := DiceGolem.ConfigureCommands(); err != nil {
		logger.Error("commands", zap.Error(err))
	}

	// if DBL token is provided, set up the background server count updater.
	if DiceGolem.TopToken != "" {
		logger.Debug("dbl enabled")
		go func() {
			for range time.Tick(10 * time.Minute) {
				postServerCount(DiceGolem)
			}
		}()
	}

	// open HTTP server for heap debugging
	go func() {
		http.ListenAndServe(":6060", nil)
	}()

	// wait 10 seconds before starting metrics
	if DiceGolem.StatsdAddr != "" {
		go func() {
			for range time.Tick(10 * time.Second) {
				emitStats(DiceGolem)
			}
		}()
	}

	// Wait here until CTRL-C or other term signal is received.
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func createFriendlyError(err error) error {
	logger.Debug("error", zap.Error(err))
	switch err {
	case dice.ErrInvalidExpression:
		return fmt.Errorf("I can't evaluate that expression. Is that roll valid?")
	case ErrNilExpressionResult:
		return fmt.Errorf("Something's wrong with that expression, it was empty.")
	case ErrTooManyDice:
		return fmt.Errorf("Your roll may require too many dice, please try a smaller roll (under %d dice).", DiceGolem.MaxDice)
	case math.ErrNilResult:
		return fmt.Errorf("Your roll didn't yield a result.")
	case ErrTokenTransition:
		return fmt.Errorf("An error was thrown when evaluating your expression. Please check for extra spaces in notations or missing math operators.")
	default:
		return fmt.Errorf("Something unexpected errored. Please check `/help`.")
	}
}

// Errors.
var (
	ErrNilExpressionResult = errors.New("nil expression result")
	ErrTokenTransition     = errors.New("token transition error")
	ErrTooManyDice         = errors.New("too many dice")
	ErrNotImplemented      = errors.New("not implemented")
)

func HandlePanic(s *discordgo.Session, i interface{}) {
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
	if r := recover(); r != nil {
		logger.Error("recovering from panic",
			zap.String("interaction", i.ID),
			zap.String("panic", fmt.Sprintf("%v", r)))
	}
	if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   1 << 6, // ephemeral
			Content: ErrUnexpectedError.Error(),
		},
	}); err != nil {
		zap.Error(err)
	}
}

// HandleReady handles a Discord READY event.
func HandleReady(s *discordgo.Session, e *discordgo.Ready) {
	logger.Info("ready received",
		zap.String("id", s.State.User.ID),
		zap.Int("shards", s.ShardCount),
		zap.Int("shard", s.ShardID),
	)
	metrics.IncrCounter([]string{"ready"}, 1)
	s.UpdateGameStatus(0, DiceGolem.Status)
}

// HandleResume handles a Discord RESUME event.
func HandleResume(s *discordgo.Session, e *discordgo.Resumed) {
	logger.Warn("resumed",
		zap.String("id", s.State.User.ID),
		zap.Int("shard", s.ShardID),
	)
	metrics.IncrCounter([]string{"resume"}, 1)
}

func HandleGuildCreate(s *discordgo.Session, e *discordgo.GuildCreate) {
	metrics.IncrCounter([]string{"guild_create"}, 1)
	logger.Debug("guild create",
		zap.Int("shard", s.ShardID),
		zap.String("id", e.ID))
}

// RouteInteractionCreate routes a Discord Interaction creation sent to the bot
// to the appropriate sub-routers ands handlers based on type.
func RouteInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	defer InteractionRecover(s, i.Interaction)
	metrics.IncrCounter([]string{"core", "interaction"}, 1)
	metrics.IncrCounter([]string{"core", "interaction", i.Type.String()}, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = NewContext(ctx, s, i.Interaction, nil)

	switch i.Type {
	// CHAT_INPUT type
	case discordgo.InteractionApplicationCommand:
		logger.Debug("interaction create",
			zap.String("name", i.ApplicationCommandData().Name),
			zap.Int("shard", s.ShardID),
			zap.Int("type", int(i.Type)),
			zap.Any("data", i),
		)
		command := i.ApplicationCommandData().Name
		defer metrics.IncrCounter([]string{"interaction", i.Type.String()}, 1)
		if handle, ok := handlers[command]; ok {
			// TODO: cache the interaction token
			// defer DiceGolem.Cache.Set(fmt.Sprintf("cache:interaction:%s:token", i.ID), i.Token, cache.DefaultExpiration)
			handle(ctx)
		} else {
			// handler doesn't exist for command
			if err := MeasureInteractionRespond(s.InteractionRespond, i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:   1 << 6,
					Content: ErrInvalidCommand.Error(),
				},
			}); err != nil {
				zap.Error(err)
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
			zap.Any("data", i),
		)
		defer metrics.IncrCounter([]string{"interaction", i.Type.String()}, 1)
		id := i.MessageComponentData().CustomID
		// if button was a macro button strip off the macro_ prefix and use the
		// ID as the rest of the expression
		if strings.HasPrefix(id, "macro_") {
			roll := strings.TrimPrefix(id, "macro_")
			_, response, _ := NewRollMessageResponseFromString(ctx, roll)
			_, err := s.ChannelMessageSendComplex(i.ChannelID, response)
			if err != nil {
				MeasureInteractionRespond(s.InteractionRespond, i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content: ErrSendMessagePermissions.Error(),
					},
				})
			} else {
				_ = MeasureInteractionRespond(s.InteractionRespond, i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseDeferredMessageUpdate,
				})
				// cacheRollExpression(s, i.Interaction, rollData.Expression)
			}
		} else if handle, ok := handlers[id]; ok {
			// if it was a generic action button, handle the press
			handle(ctx)
		} else {
			// handler doesn't exist for sent command
			if err := MeasureInteractionRespond(s.InteractionRespond, i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:   1 << 6,
					Content: ErrInvalidCommand.Error(),
				},
			}); err != nil {
				zap.Error(err)
			}
			return
		}

	// Auto-complete events with users' partial input data
	case discordgo.InteractionApplicationCommandAutocomplete:
		option := getFocusedOption(i.ApplicationCommandData()).Name
		defer metrics.IncrCounter([]string{"interaction", i.Type.String()}, 1)
		defer metrics.MeasureSince([]string{"core", "autocomplete"}, time.Now())
		if suggest, ok := suggesters[option]; ok {
			suggest(ctx)
		}
	}
}

func HandleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	defer HandlePanic(s, m.Message)
	defer metrics.IncrCounter([]string{"message_in"}, 1)
	logger.Debug("message_in", zap.Any("message", m))
	// no content means there's no roll text to process
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Debug("handle message",
		zap.String("id", m.ID),
		zap.String("chan", m.ChannelID),
		zap.String("guild", m.GuildID),
		zap.Int("shard", s.ShardID),
	)

	ctx = NewContext(ctx, s, nil, m.Message)
	res, message, rollErr := NewMessageResponseFromMessage(ctx, m.Message)
	// if nothing to send, short-circuit
	if message == nil {
		return
	}

	// if roll was OK, cache it
	if rollErr == nil && res != nil {
		roll := (&RollInput{
			Expression: res.Expression,
			Label:      res.Label,
		}).Serialize()
		defer DiceGolem.Cache.SetWithTTL(fmt.Sprintf(CacheKeyMessageDataFormat, m.ID), roll, DiceGolem.CacheTTL)
	}

	// cache expression for response as well
	resMessage, resErr := s.ChannelMessageSendComplex(m.ChannelID, message)
	if res != nil && resErr == nil && rollErr == nil {
		roll := (&RollInput{
			Expression: res.Expression,
			Label:      res.Label,
		}).Serialize()
		defer DiceGolem.Cache.SetWithTTL(fmt.Sprintf(CacheKeyMessageDataFormat, resMessage.ID), roll, DiceGolem.CacheTTL)
	}

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
	if err := DiceGolem.EmitNotificationMessage(&discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{
			{
				Description: fmt.Sprintf("Rate limit hit on shard %d", s.ShardID),
				Color:       0xed4245, // rgb(237, 66, 69)
			},
		},
	}); err != nil {
		logger.Error("notify", zap.Error(err))
	}
}
