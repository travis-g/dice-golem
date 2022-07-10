package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
)

var (
	// A map of handlers for Discord Interactions. There should be a handler for
	// every static command. Keys should not be removed until after the 1-hour
	// grace period following changes to the bot's ApplicationCommand lists.
	// TODO: forward context into the handlers
	handlers = map[string]func(ctx context.Context){
		"roll":    RollInteractionCreate,
		"secret":  RollInteractionCreateEphemeral,
		"private": RollInteractionCreatePrivate,
		"help": func(ctx context.Context) {
			s, i, _ := FromContext(ctx)
			if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: 1 << 6,
					Embeds: []*discordgo.MessageEmbed{
						makeEmbedHelp(),
					},
				},
			}); err != nil {
				if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content: "Something went wrong!",
					},
				}); err != nil {
					zap.Error(err)
				}
				return
			}
		},
		"info": func(ctx context.Context) {
			s, i, _ := FromContext(ctx)

			if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: 1 << 6,
					Embeds: []*discordgo.MessageEmbed{
						makeEmbedInfo(),
					},
				},
			}); err != nil {
				if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content: "Something went wrong!",
					},
				}); err != nil {
					zap.Error(err)
				}
				return
			}
		},

		"buttons": ButtonsInteraction,
		"ping":    PingInteraction,
		"clear":   ClearInteraction,

		"settings": SettingsInteraction,

		// home-server commands
		"state": StateInteraction,
		"stats": StatsInteraction,

		// message commands
		"Roll Message": RollMessageInteractionCreate,
		// "Roll Message (Secret)":
		// "Roll Message (Private)":
		// "Save Macro":
	}

	suggesters = map[string]func(ctx context.Context){
		"expression": SuggestExpression,
		"label":      SuggestLabel,
	}
)

func MakeApplicationCommandOptions(optionSets ...[]*discordgo.ApplicationCommandOption) []*discordgo.ApplicationCommandOption {
	var newOpts = []*discordgo.ApplicationCommandOption{}
	for _, optionSet := range optionSets {
		newOpts = append(newOpts, optionSet...)
	}
	return newOpts
}

// RollInteractionCreate is the method evaluated against a chat command to roll
// dice.
func RollInteractionCreate(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	logger.Info("interaction", zap.String("id", i.ID))
	logger.Debug("interaction data", zap.Any("data", i.ApplicationCommandData()))

	options := i.ApplicationCommandData().Options
	// forward to other roll interaction handlers if certain options are set
	if optPrivate := getOptionByName(options, "private"); optPrivate != nil && optPrivate.BoolValue() {
		RollInteractionCreatePrivate(ctx)
		return // short circuit
	} else if optSecret := getOptionByName(options, "secret"); optSecret != nil && optSecret.BoolValue() {
		RollInteractionCreateEphemeral(ctx)
		return // short circuit
	}

	rollData, response, rollErr := NewRollInteractionResponseFromInteraction(ctx)
	if response == nil {
		return
	}

	user := UserFromInteraction(i)
	if rollErr == nil {
		roll := &RollInput{
			Expression: rollData.Expression,
			Label:      rollData.Label,
		}
		defer cacheRollInput(s, i, roll)
		defer CacheRoll(user, roll)
	}
	if err := MeasureInteractionRespond(s.InteractionRespond, i, response); err != nil {
		logger.Error("roll interaction error", zap.Error(err))
	}
}

// RollInteractionCreateEphemeral is the method evaluated against an interaction to roll
// dice.
func RollInteractionCreateEphemeral(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	logger.Info("interaction", zap.String("id", i.ID))
	logger.Debug("interaction data", zap.Any("data", i.ApplicationCommandData()))

	_, response, _ := NewRollInteractionResponseFromInteraction(ctx)
	if response == nil {
		return
	}
	// Tweak the InteractionResponse to be ephemeral
	response.Data.Flags = 1 << 6
	if err := MeasureInteractionRespond(s.InteractionRespond, i, response); err != nil {
		zap.Error(err)
		return
	}
}

// RollInteractionCreatePrivate is the method evaluated against an interaction
// to roll dice but to DM the user the result.
func RollInteractionCreatePrivate(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	logger.Info("interaction", zap.String("id", i.ID))
	logger.Debug("interaction data", zap.Any("data", i.ApplicationCommandData()))

	// check out who/where the roll was sent
	var uid string
	if i.Member != nil {
		uid = i.Member.User.ID
	} else {
		// if no member data, this was in a DM channel...just send an interaction!
		RollInteractionCreate(ctx)
		return
	}

	_, response, _ := NewRollInteractionResponseFromInteraction(ctx)
	if response == nil {
		return
	}

	// create a DM channel, but since we can't respond as an interaction across
	// channels convert the response to a regular message
	c, _ := DiceGolem.DefaultSession.UserChannelCreate(uid)
	m := messageSendFromInteraction(response)
	_, err := DiceGolem.DefaultSession.ChannelMessageSendComplex(c.ID, m)
	if err != nil {
		MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   1 << 6, // ephemeral
				Content: "Sorry! A direct message couldn't be sent. Do you allow DMs?",
			}})
		return
	}

	if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Sent you a DM!",
			Flags:   1 << 6,
		},
	}); err != nil {
		zap.Error(err)
		return
	}
}

// RollMessageInteractionCreate is called by interaction to roll a message's
// content using the 'Apps' context option. The cache should be checked to
// determine if a message's associated input expression is already available.
func RollMessageInteractionCreate(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	targetMessage := i.ApplicationCommandData().Resolved.Messages[i.ApplicationCommandData().TargetID]
	defer metrics.IncrCounter([]string{"roll_message"}, 1)
	logger.Info("interaction", zap.String("id", i.ID), zap.String("target", targetMessage.ID))
	logger.Debug("interaction data", zap.Any("data", i.ApplicationCommandData()))

	// the expression to roll
	var input string
	// check cache
	key := fmt.Sprintf(CacheKeyMessageDataFormat, targetMessage.ID)
	cachedSerial, ok := DiceGolem.Cache.Get(key)
	if ok {
		input = cachedSerial.(string)
	} else {
		// if not in cache, try evaluating the message content
		// TODO: fuzzy-extract an expression
		logger.Debug("cache miss", zap.String("key", key))
		// TODO: handle seprately if interaction was not pulled from cache
		input = targetMessage.Content
	}

	// TODO: clean up input/extract roll from between accents, etc.

	rollData, interactionResponse, err := NewRollInteractionResponseFromStringWithContext(ctx, input)
	if interactionResponse == nil {
		return
	}

	user := UserFromInteraction(i)

	if err == nil {
		roll := &RollInput{
			Expression: rollData.Expression,
			Label:      rollData.Label,
		}
		defer cacheRollInput(s, i, roll)
		defer CacheRoll(user, roll)
	}

	if resErr := MeasureInteractionRespond(s.InteractionRespond, i, interactionResponse); resErr != nil {
		zap.Error(resErr)
		return
	}
}

// NewRollInteractionResponseFromInteraction is the method evaluated against an
// Interaction to roll dice and create the basic response object.
func NewRollInteractionResponseFromInteraction(ctx context.Context) (*Response, *discordgo.InteractionResponse, error) {
	_, i, _ := FromContext(ctx)
	options := i.ApplicationCommandData().Options
	expression := options[0].StringValue()
	input := NewRollInputFromString(expression)

	// check if we entered with a no-op expression
	if input.Expression == "" {
		return nil, nil, nil
	}

	message, response, err := NewRollInteractionResponseFromStringWithContext(ctx, input.Serialize())
	if err != nil {
		return message, response, err
	}

	detailed := IsSet(UserFromInteraction(i), Detailed)
	optDetailed := getOptionByName(options, "detailed")
	if optDetailed != nil {
		detailed = optDetailed.BoolValue()
	}

	if detailed {
		response.Data.Embeds = []*discordgo.MessageEmbed{{
			// FIXME: return a markdown string
			Description: MarkdownDetails(ctx, message.Dice),
		}}
	}

	return message, response, err
}

// NewRollInteractionResponseFromStringWithContext creates an Interaction
// response and roll response. If an error occurred the error will be returned,
// but the returned InteractionResponse will be an error message response to be
// sent back to Discord.
func NewRollInteractionResponseFromStringWithContext(ctx context.Context, expression string) (*Response, *discordgo.InteractionResponse, error) {
	s, i, _ := FromContext(ctx)
	if s == nil || i == nil {
		panic(errors.New("context data missing"))
	}

	// add expression to context
	roll := NewRollInputFromString(expression)
	ctx = context.WithValue(ctx, KeyRollInput, roll)

	// check for excessive dice
	if excessiveDice(ctx) {
		return nil, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   1 << 6, // ephemeral
				Content: createFriendlyError(ErrTooManyDice).Error(),
			},
		}, ErrTooManyDice
	}

	options := i.ApplicationCommandData().Options

	// if a Slash command, check for a label
	if i.Type == discordgo.InteractionApplicationCommand {
		optLabel := getOptionByName(options, "label")
		if optLabel != nil {
			roll.Label = optLabel.StringValue()
		}
	}

	message, err := EvaluateRollInputWithContext(ctx, roll)
	if err != nil {
		// TODO: better error handling
		logger.Info("error response", zap.String("msg", createFriendlyError(err).Error()))
		return nil, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   1 << 6, // ephemeral
				Content: createFriendlyError(err).Error(),
			},
		}, err
	}

	mentionableUserIDs := []string{}
	if i.Member != nil {
		// add user's name if roll is shared to a guild channel
		if isRollPublic(i) {
			message.Name = i.Member.Mention()
		}
		// allow mentioning only the user that requested the roll even if others
		// are @mentioned (ex. '/roll expression:"3d6" label:"vs @travis' AC"')
		mentionableUserIDs = append(mentionableUserIDs, i.Member.User.ID)
	}

	// build the message content using a template
	var text strings.Builder
	responseResultTemplateCompiled.Execute(&text, message)

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: text.String(),
			AllowedMentions: &discordgo.MessageAllowedMentions{
				Users: []string{},
				// Users: mentionableUserIDs,
			},
		},
	}

	// get user's default preference
	detailed := IsSet(UserFromInteraction(i), Detailed)
	if optDetailed := getOptionByName(options, "detailed"); optDetailed != nil {
		detailed = optDetailed.BoolValue()
	}
	if detailed {
		response.Data.Embeds = []*discordgo.MessageEmbed{{
			Description: MarkdownDetails(ctx, message.Dice),
		}}
	}

	return message, response, nil
}

// NewRollMessageResponse is a wrapper for NewRollMessageResponseFromString that
// uses the message's Content.
func NewMessageResponseFromMessage(ctx context.Context, m *discordgo.Message) (*Response, *discordgo.MessageSend, error) {
	return NewRollMessageResponseFromString(ctx, m.Content)
}

// NewRollMessageResponseFromString takes a message's content, lints it, and
// evaluates it as a roll. A bot response and Discord message to send as the
// response to the roll will be returned.
func NewRollMessageResponseFromString(ctx context.Context, content string) (*Response, *discordgo.MessageSend, error) {
	if content == "" {
		return nil, nil, nil
	}

	_, i, m := FromContext(ctx)

	// strip out bot mentions and clean the roll up
	content = strings.NewReplacer(
		"<@"+DiceGolem.User.ID+">", "@"+DiceGolem.User.Username,
		"<@!"+DiceGolem.User.ID+">", "@"+DiceGolem.User.Username,
	).Replace(content)
	input := strings.TrimSpace(strings.ReplaceAll(content, "@"+DiceGolem.User.Username, ""))
	roll := NewRollInputFromString(input)

	// if message is empty, do nothing
	if roll.Expression == "" {
		return nil, nil, nil
	}

	// add roll to context
	ctx = context.WithValue(ctx, KeyRollInput, roll)

	logger.Debug("data", zap.String("content", content), zap.Any("roll", roll))

	res, err := EvaluateRollInputWithContext(ctx, roll)
	if err != nil {
		return res, &discordgo.MessageSend{
			Content: createFriendlyError(err).Error(),
			Reference: &discordgo.MessageReference{
				MessageID: m.ID,
				ChannelID: m.ChannelID,
			},
		}, err
	}

	var user *discordgo.User
	// if in a guild @mention the user
	if m != nil && m.Author != nil && m.GuildID != "" {
		user = m.Author
		res.Name = user.Mention()
	} else if m != nil {
		user = UserFromMessage(m)
	} else if i != nil {
		user = UserFromInteraction(i)
		if i.GuildID != "" {
			res.Name = user.Mention()
		}
	}

	var text strings.Builder
	responseResultTemplateCompiled.Execute(&text, res)

	message := &discordgo.MessageSend{
		Content: text.String(),
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Users: []string{},
		},
	}

	if IsSet(user, Detailed) {
		message.Embeds = []*discordgo.MessageEmbed{{
			Description: MarkdownDetails(ctx, res.Dice),
		}}
	}

	return res, message, nil
}

// PingInteraction is the handler for checking the bot's rount-trip time with
// Discord.
func PingInteraction(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	start := time.Now()
	if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: 1 << 6,
		},
	}); err != nil {
		logger.Error("ping", zap.Error(err))
	}
	done := time.Now()
	up := done.Sub(start)
	// get message
	m, _ := s.InteractionResponseEdit(i, &discordgo.WebhookEdit{})
	fetched := time.Now()
	logger.Debug("response message", zap.Any("message", m))
	down := fetched.Sub(done)
	if _, err := s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
		Content: " ",
		Embeds: []*discordgo.MessageEmbed{
			{
				Fields: []*discordgo.MessageEmbedField{
					{
						Name: "Ping",
						Value: fmt.Sprintf("%s (%s, %s)",
							fetched.Sub(start).Round(time.Millisecond).String(),
							up.Round(time.Millisecond).String(),
							down.Round(time.Millisecond).String()),
						Inline: true,
					},
					{
						Name:   "Heartbeat",
						Value:  s.HeartbeatLatency().Round(time.Millisecond).String(),
						Inline: true,
					},
					{
						Name:   "Shard",
						Value:  strconv.Itoa(s.ShardID),
						Inline: true,
					},
				},
				Footer: &discordgo.MessageEmbedFooter{
					Text:    DiceGolem.DefaultSession.State.User.Username,
					IconURL: DiceGolem.DefaultSession.State.User.AvatarURL("64"),
				},
			},
		},
	}); err != nil {
		logger.Error("interaction response", zap.Error(err))
		return
	}
}

// ButtonsInteraction sends a macro pad of common dice. Presses are handled
// programmatically by HandleInteractionCreate.
func ButtonsInteraction(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	logger.Debug("buttons handler called", zap.String("interaction", i.ID))

	// dm := i.Member != nil
	// uid := UserFromInteraction(i).ID

	// c, _ := s.UserChannelCreate(uid)
	// logger.Debug("created channel", zap.String("id", c.ID))

	// _, err := s.ChannelMessageSendComplex(c.ID, &discordgo.MessageSend{
	// 	Content:    "<:dice_golem:741798570289660004> Dice buttons!",
	// 	Components: DefaultPadComponents,
	// })
	// if err != nil {
	// 	logger.Error("err creating DM channel", zap.Error(err))
	// }

	errRes := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Sorry! I couldn't create a direct message with you. Do you allow DMs?",
		},
	}

	// if err != nil {
	// 	MeasureInteractionRespond(s.InteractionRespond, i, errRes)
	// 	return
	// }
	err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    "<:dice_golem:741798570289660004> Dice buttons!",
			Flags:      1 << 6,
			Components: DefaultPadComponents,
		},
	})

	// only send the tip message if we're not already in a DM
	// if err == nil && !dm {
	// 	MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
	// 		Type: discordgo.InteractionResponseChannelMessageWithSource,
	// 		Data: &discordgo.InteractionResponseData{
	// 			Content: "Sent you a direct message!",
	// 			Flags:   1 << 6,
	// 		},
	// 	})
	// } else
	if err != nil {
		MeasureInteractionRespond(s.InteractionRespond, i, errRes)
		return
	}
}

// StateInteraction sends bot state information.
func StateInteraction(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	logger.Debug("state handler called", zap.String("interaction", i.ID))

	if !DiceGolem.IsOwner(UserFromInteraction(i)) {
		if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   1 << 6,
				Content: "This command is reserved for bot administrators.",
			},
		}); err != nil {
			zap.Error(err)
		}
	} else {
		if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: makeStateEmbed(),
			},
		}); err != nil {
			zap.Error(err)
		}
	}

}

// StatsInteraction sends bot stats.
func StatsInteraction(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	logger.Debug("stats handler called", zap.String("interaction", i.ID))

	if !DiceGolem.IsOwner(i.Member.User) {
		if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   1 << 6,
				Content: "This command is reserved for bot administrators.",
			},
		}); err != nil {
			zap.Error(err)
		}
	} else {
		if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: makeStatsEmbed(),
			},
		}); err != nil {
			zap.Error(err)
		}
	}
}

func ClearInteraction(ctx context.Context) {
	s, i, _ := FromContext(ctx)

	options := i.ApplicationCommandData().Options
	u := UserFromInteraction(i)
	switch options[0].Name {
	case "recent":
		// clear out recent roll key from the cache
		if DiceGolem.Redis != nil {
			key := fmt.Sprintf(CacheKeyUserRecentFormat, u.ID)
			DiceGolem.Redis.Del(key)
		}
		if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Cleared your cached roll history (if any).",
				Flags:   1 << 6,
			},
		}); err != nil {
			zap.Error(err)
		}
	default:
		if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Sorry, an invalid subcommand was received!",
				Flags:   1 << 6,
			},
		}); err != nil {
			logger.Error("subcommand error", zap.Error(err))
		}
	}
}

func SettingsInteraction(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	user := UserFromInteraction(i)
	options := i.ApplicationCommandData().Options
	switch options[0].Name {
	case "recent":
		options = options[0].Options
		switch options[0].Name {
		case "enable":
			Unset(user, NoRecent)
		case "disable":
			Set(user, NoRecent)
			DiceGolem.Redis.Del(fmt.Sprintf(CacheKeyUserRecentFormat, user.ID))
		}
	case "detailed":
		options = options[0].Options
		switch options[0].Name {
		case "enable":
			Set(user, Detailed)
		case "disable":
			Unset(user, Detailed)
		}
	}
	s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   1 << 6,
			Content: "Updated your settings.",
		},
	})
}

func MeasureInteractionRespond(fn func(*discordgo.Interaction, *discordgo.InteractionResponse) error, i *discordgo.Interaction, r *discordgo.InteractionResponse) error {
	defer metrics.MeasureSince([]string{"interaction", "send"}, time.Now())
	return fn(i, r)
}

type interactionResponse struct {
	ID string `json:"id,omitempty"`
}

// GetInteractionResponse retrieves the response Message for an Interaction sent
// by the bot. Since Discord's API doesn't return the Message when sent, we
// have to manually fetch it.
func GetInteractionResponse(s *discordgo.Session, i *discordgo.Interaction) (id string, err error) {
	uri := discordgo.EndpointWebhookMessage(s.State.User.ID, i.Token, "@original")

	body, err := s.RequestWithBucketID("GET", uri, nil, discordgo.EndpointWebhookToken("", ""))
	if err != nil {
		logger.Error("interaction response fetch", zap.Error(err), zap.String("interaction", i.ID))
		return "", err
	}
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		logger.Error("failed to unmarshal message", zap.Error(err))
		return "", err
	}
	logger.Debug("interaction response", zap.String("interaction", i.ID), zap.Any("body", data))
	return data["id"].(string), nil
}

func UserFromInteraction(i *discordgo.Interaction) (user *discordgo.User) {
	if i == nil {
		return nil
	}

	if i.Member != nil {
		return i.Member.User
	}
	return i.User
}

func UserFromMessage(m *discordgo.Message) (user *discordgo.User) {
	logger.Debug("user_from_message", zap.Any("message", m))
	if m == nil {
		return nil
	}

	return m.Author
}

// isRollPublic returns whether a roll-like interaction would be shown to a
// guild channel.
func isRollPublic(i *discordgo.Interaction) bool {
	if !contains([]string{"roll", "Roll Message"}, i.ApplicationCommandData().Name) {
		return false
	}

	// no guild member data, therefore from a DM
	if i.Member == nil {
		return false
	}

	options := i.ApplicationCommandData().Options
	if optEphemeral := getOptionByName(options, "secret"); optEphemeral != nil && optEphemeral.BoolValue() {
		return false
	}
	if optPrivate := getOptionByName(options, "private"); optPrivate != nil && optPrivate.BoolValue() {
		return false
	}
	return true
}
