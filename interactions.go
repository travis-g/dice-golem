package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/gocarina/gocsv"
	"go.uber.org/zap"
)

var (
	// A map of handlers for Discord Interactions. There should be a handler for
	// every static command. Keys should not be removed until after the 1-hour
	// grace period following changes to the bot's ApplicationCommand lists.
	handlers = map[string]func(ctx context.Context){
		"roll":    RollInteractionCreate,
		"secret":  RollInteractionCreateEphemeral,
		"private": RollInteractionCreatePrivate,
		"help": func(ctx context.Context) {
			s, i, _ := FromContext(ctx)
			if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{
						makeEmbedHelp(),
					},
				},
			}); err != nil {
				if err := MeasureInteractionRespond(s.InteractionRespond, i,
					newEphemeralResponse("Something went wrong!")); err != nil {
					logger.Error("error sending response", zap.Error(err))
				}
				return
			}
		},
		"info": func(ctx context.Context) {
			s, i, _ := FromContext(ctx)

			if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{
						makeEmbedInfo(),
					},
				},
			}); err != nil {
				logger.Error("error sending info", zap.Error(err))
				if err := MeasureInteractionRespond(s.InteractionRespond, i,
					newEphemeralResponse("Something went wrong!"),
				); err != nil {
					logger.Error("error sending response", zap.Error(err))
				}
				return
			}
		},

		"invite": InteractionAdd, // DEPRECATED
		"add":    InteractionAdd,

		"expressions": InteractionExpressions,

		"buttons": InteractionButtons,
		"ping":    InteractionPing,
		"clear":   InteractionClear,

		"preferences": InteractionPreferences,
		"settings":    InteractionSettings,

		// home-server commands
		"health":    InteractionHealth,
		"stats":     InteractionStats,
		"golemancy": InteractionGolemancy,
		"debug":     InteractionDebug,

		// message commands
		"Roll Message":    RollMessageInteractionCreate,
		"Save Expression": SaveRollInteractionCreate,
	}

	suggesters = map[string]func(ctx context.Context){
		"roll:expression":               SuggestRolls,
		"secret:expression":             SuggestRolls,
		"private:expression":            SuggestRolls,
		"roll:label":                    SuggestLabel,
		"secret:label":                  SuggestLabel,
		"private:label":                 SuggestLabel,
		"buttons modifiers:expression":  SuggestRolls,
		"expressions save:expression":   SuggestExpressions,
		"expressions save:label":        SuggestLabel,
		"expressions save:name":         SuggestNames,
		"expressions unsave:expression": SuggestNames,
	}
)

// MergeApplicationCommandOptions appends a number of sets of command options
// together.
func MergeApplicationCommandOptions(optionSets ...[]*discordgo.ApplicationCommandOption) []*discordgo.ApplicationCommandOption {
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
	logger.Info("interaction", zap.String("id", i.ID), zap.Int("shard", s.ShardID))
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

	// count regular roll
	defer metrics.IncrCounter([]string{"roll", "basic"}, 1)

	rollData, response, rollErr := NewRollInteractionResponseFromInteraction(ctx)
	if response == nil {
		return
	}

	user := UserFromInteraction(i)
	if rollErr == nil && len(rollData.Dice) > 0 {
		roll := &NamedRollInput{
			Expression: rollData.Expression,
			Label:      rollData.Label,
		}
		// defer cacheRollInput(s, i, roll)
		defer CacheRoll(user, roll)
	}

	// TODO: check forwarding configuration

	if err := MeasureInteractionRespond(s.InteractionRespond, i, response); err != nil {
		logger.Error("roll interaction error", zap.Error(err))
	}
}

// RollInteractionCreateEphemeral is the method evaluated against an interaction to roll
// dice.
func RollInteractionCreateEphemeral(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	logger.Info("interaction", zap.String("id", i.ID), zap.Int("shard", s.ShardID))
	logger.Debug("interaction data", zap.Any("data", i.ApplicationCommandData()))

	rollData, response, rollErr := NewRollInteractionResponseFromInteraction(ctx)
	if response == nil {
		return
	}

	user := UserFromInteraction(i)
	if rollErr == nil {
		roll := &NamedRollInput{
			Expression: rollData.Expression,
			Label:      rollData.Label,
		}
		defer CacheRoll(user, roll)
	}

	// count secret/ephemeral roll
	defer metrics.IncrCounter([]string{"roll", "ephemeral"}, 1)

	// Tweak the InteractionResponse to be ephemeral
	response.Data.Flags = discordgo.MessageFlagsEphemeral
	if err := MeasureInteractionRespond(s.InteractionRespond, i, response); err != nil {
		logger.Error("error sending response", zap.Error(err))
		return
	}
}

// RollInteractionCreatePrivate is the method evaluated against an interaction
// to roll dice but to DM the user the result.
func RollInteractionCreatePrivate(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	logger.Info("interaction", zap.String("id", i.ID), zap.Int("shard", s.ShardID))
	logger.Debug("interaction data", zap.Any("data", i.ApplicationCommandData()))

	uid := UserFromInteraction(i).ID

	rollData, response, rollErr := NewRollInteractionResponseFromInteraction(ctx)
	if response == nil {
		return
	}

	user := UserFromInteraction(i)
	if rollErr == nil {
		roll := &NamedRollInput{
			Expression: rollData.Expression,
			Label:      rollData.Label,
		}
		defer CacheRoll(user, roll)
	}

	// create a DM channel, but since we can't respond as an interaction across
	// channels convert the response to a regular message
	c, _ := s.UserChannelCreate(uid)
	m := newMessageSendFromInteractionResponse(response)
	_, err := s.ChannelMessageSendComplex(c.ID, m)
	if err != nil {
		MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse(ErrDMError.Error()))
		return
	}

	// count private roll
	defer metrics.IncrCounter([]string{"roll", "private"}, 1)

	if err := MeasureInteractionRespond(s.InteractionRespond, i,
		newEphemeralResponse("Sent you a DM!")); err != nil {
		logger.Error("error sending response", zap.Error(err))
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
	input := targetMessage.Content

	// TODO: clean up input/extract roll from between accents, etc.

	rollData, interactionResponse, err := NewRollInteractionResponseFromStringWithContext(ctx, input)
	if interactionResponse == nil {
		return
	}

	user := UserFromInteraction(i)

	if err == nil && len(rollData.Dice) > 0 {
		roll := &NamedRollInput{
			Expression: rollData.Expression,
			Label:      rollData.Label,
		}
		// defer cacheRollInput(s, i, roll)
		defer CacheRoll(user, roll)
	}

	if resErr := MeasureInteractionRespond(s.InteractionRespond, i, interactionResponse); resErr != nil {
		zap.Error(resErr)
		return
	}
}

func SaveRollInteractionCreate(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	defer metrics.IncrCounter([]string{"interaction", "save_message"}, 1)
	targetMessage := i.ApplicationCommandData().Resolved.Messages[i.ApplicationCommandData().TargetID]
	logger.Debug("interaction data", zap.Any("data", i.ApplicationCommandData()))

	// the expression to roll
	input := targetMessage.Content

	seed := NewRollInputFromString(input)
	modal := makeSaveExpressionModal(seed)
	if err := MeasureInteractionRespond(s.InteractionRespond, i, modal); err != nil {
		logger.Error("modal send", zap.Error(err))
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

	message, response, err := NewRollInteractionResponseFromStringWithContext(ctx, input.RollableString())
	if err != nil {
		return message, response, err
	}

	detailed := UserHasPreference(UserFromInteraction(i), SettingDetailed)
	optDetailed := getOptionByName(options, "detailed")
	if optDetailed != nil {
		detailed = optDetailed.BoolValue()
	}

	if detailed {
		response.Data.Embeds = MessageEmbeds(ctx, &RollLog{
			Entries: []*Response{message},
		})
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
		panic("context data missing")
	}

	// add expression to context
	roll := NewRollInputFromString(expression)
	ctx = context.WithValue(ctx, KeyRollInput, roll)

	// check for excessive dice
	if excessiveDice(ctx) {
		return nil, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral, // ephemeral
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
				Flags:   discordgo.MessageFlagsEphemeral, // ephemeral
				Content: createFriendlyError(err).Error(),
			},
		}, err
	}

	// mentionableUserIDs := []string{}
	if i.Member != nil {
		// add user's name if roll is shared to a guild channel
		if isInteractionPublic(i) {
			message.Name = UserFromInteraction(i).Mention()
		}
		// allow mentioning only the user that requested the roll even if others
		// are @mentioned (ex. '/roll expression:"3d6" label:"vs @travis' AC"')
		// mentionableUserIDs = append(mentionableUserIDs, UserFromInteraction(i).ID)
	}

	// build the message content using a template
	logger.Debug("rendering response", zap.Any("message", message))
	var text strings.Builder
	_ = responseResultTemplateCompiled.Execute(&text, message)

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
	detailed := UserHasPreference(UserFromInteraction(i), SettingDetailed)
	if optDetailed := getOptionByName(options, "detailed"); optDetailed != nil {
		detailed = optDetailed.BoolValue()
	}
	if detailed {
		response.Data.Embeds = MessageEmbeds(ctx, &RollLog{
			Entries: []*Response{message},
		})
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
		"<@"+DiceGolem.SelfID+">", "",
		"<@!"+DiceGolem.SelfID+">", "",
	).Replace(content)
	input := strings.TrimSpace(content)
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
		// if in a DM skip the user mention/res.Name
		user = UserFromMessage(m)
	} else if i != nil {
		user = UserFromInteraction(i)
		if i.GuildID != "" {
			res.Name = user.Mention()
		}
	}

	var text strings.Builder
	executeResponseTemplate(&text, res)

	message := &discordgo.MessageSend{
		Content: text.String(),
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Users: []string{},
		},
	}

	if UserHasPreference(user, SettingDetailed) {
		message.Embeds = MessageEmbeds(ctx, &RollLog{
			Entries: []*Response{res},
		})
	}

	return res, message, nil
}

// InteractionPing is the handler for checking the bot's rount-trip time with
// Discord.
func InteractionPing(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	start := time.Now()
	// ACK the ping
	if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		logger.Error("ping", zap.Error(err))
	}
	// measure time to ACK write
	done := time.Now()
	up := done.Sub(start)

	// get message
	var m *discordgo.Message
	var err error
	if m, err = s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
		Content: Ptr(" "),
	}); err != nil {
		logger.Error("ping", zap.Error(err))
	}

	// measure GET time
	fetched := time.Now()
	down := fetched.Sub(done)
	logger.Debug("response message", zap.Any("message", m))
	avg := (up + down) / 2
	embeds := []*discordgo.MessageEmbed{
		{
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Heartbeat",
					Value:  s.HeartbeatLatency().Round(time.Millisecond).String(),
					Inline: true,
				},
				{
					Name: "API",
					Value: fmt.Sprintf("%s (%s ↑, %s ↓)",
						avg.Round(time.Millisecond).String(),
						up.Round(time.Millisecond).String(),
						down.Round(time.Millisecond).String()),
					Inline: true,
				},
				{
					Name:   "Shard",
					Value:  strconv.Itoa(s.ShardID),
					Inline: true,
				},
			},
			Footer: makeEmbedFooter(),
		},
	}
	if _, err := s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
		Content: Ptr(" "),
		Embeds:  &embeds,
	}); err != nil {
		logger.Error("interaction response", zap.Error(err))
		return
	}
}

func InteractionAdd(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label: "Add App",
							Style: discordgo.LinkButton,
							URL:   add,
							Emoji: &discordgo.ComponentEmoji{
								Name: "dice_golem",
								ID:   "1031958619782127616",
							},
						},
						discordgo.Button{
							Label: "App Directory",
							Style: discordgo.LinkButton,
							URL:   directory,
						},
					},
				},
			},
		},
	}); err != nil {
		logger.Error("add error", zap.Error(err))
		if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Sorry, a link couldn't be sent! Check the bot's Discord profile for alternative links.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			logger.Error("add error", zap.Error(err))
		}
	}
}

func InteractionExpressions(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	logger.Debug("expressions handler called", zap.String("interaction", i.ID))

	subcommand := i.ApplicationCommandData().Options
	u := UserFromInteraction(i)
	switch subcommand[0].Name {
	case "save":
		key := fmt.Sprintf(KeyCacheUserGlobalExpressionsFmt, u.ID)
		count := DiceGolem.Cache.Redis.HLen(ctx, key).Val()
		if DiceGolem.Cache.Redis != nil && count >= int64(DiceGolem.MaxExpressions) {
			MeasureInteractionRespond(s.InteractionRespond, i,
				newEphemeralResponse(fmt.Sprintf("You already have the maximum of %d saved expressions. Please remove one before adding another.", DiceGolem.MaxExpressions)),
			)
			return
		}
		options := subcommand[0].Options
		var roll = new(NamedRollInput)
		if optExpression := getOptionByName(options, "expression"); optExpression != nil {
			roll.Expression = optExpression.StringValue()
		} else {
			panic("expression required")
		}
		if optName := getOptionByName(options, "name"); optName != nil {
			roll.Name = optName.StringValue()
		}
		if optLabel := getOptionByName(options, "label"); optLabel != nil {
			roll.Label = optLabel.StringValue()
		}
		roll.Clean()
		if err := roll.Validate(); err != nil {
			MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Expression is invalid: "+err.Error()))
			return
		}
		if err := SetNamedRoll(u, i.GuildID, roll); err != nil {
			logger.Error("error saving roll", zap.Error(err))
			MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Something unexpected errored! Please try again later."))
			return
		}
		MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse(fmt.Sprintf("Saved `%v`! Total expressions: %d", roll, count+1)))
	case "unsave":
		key := fmt.Sprintf(KeyCacheUserGlobalExpressionsFmt, u.ID)
		if DiceGolem.Cache.Redis != nil {
			if optExpression := getOptionByName(subcommand[0].Options, "expression"); optExpression != nil {
				num, err := DiceGolem.Cache.Redis.HDel(ctx, key, optExpression.StringValue()).Result()
				if err != nil {
					panic(err)
				}
				if num == 1 {
					MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Removed the expression."))
				}
			}
			return
		}
	case "export":
		rolls, _ := GetNamedRolls(UserFromInteraction(i), "")
		if len(rolls) == 0 {
			MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("You don't have any saved expressions."))
			return
		}
		if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "Exported your saved expressions to a CSV. Be sure to download it!",
				Files: []*discordgo.File{
					ExportExpressions(ctx, rolls),
				},
			},
		}); err != nil {
			logger.Error("error sending export", zap.Error(err))
			MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Something unexpected errored! The bot may be missing the _Attach Files_ permission."))
			return
		}
	case "edit":
		rolls, _ := GetNamedRolls(UserFromInteraction(i), "")
		csvBytes, err := gocsv.MarshalBytes(&rolls)
		if err != nil {
			MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Something unexpected errored!"))
			return
		}
		modal := makeEditExpressionsModal(string(csvBytes))
		if err := MeasureInteractionRespond(s.InteractionRespond, i, modal); err != nil {
			logger.Error("error sending modal", zap.Error(err))
			MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Something unexpected errored!"))
			return
		}
	case "clear":
		_ = InteractionExpressionsClear(ctx, u)
		if err := MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Cleared your saved expressions (if any).")); err != nil {
			logger.Error("error sending response", zap.Error(err))
		}
	default:
		MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Sorry! That subcommand does not have a handler yet."))
	}
}

// InteractionButtons sends a macro pad of common dice. Presses are handled
// programmatically by HandleInteractionCreate.
func InteractionButtons(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	logger.Debug("buttons handler called", zap.String("interaction", i.ID))

	errRes := newEphemeralResponse(ErrDMError.Error())

	// if err != nil {
	// 	MeasureInteractionRespond(s.InteractionRespond, i, errRes)
	// 	return
	// }

	subcommand := i.ApplicationCommandData().Options
	var components []discordgo.MessageComponent
	switch subcommand[0].Name {
	case "dnd5e":
		components = Dnd5ePadComponents
	case "d20":
		components = D20PadComponents
	case "fate":
		components = FatePadComponents
	case "modifiers":
		options := i.ApplicationCommandData().Options
		base := mustGetOptionByName(options, "expression")
		optLowest := getOptionByName(options, "lowest")
		optHighest := getOptionByName(options, "highest")
		var lowest, highest int
		if optLowest == nil && optHighest == nil {
			lowest = -7
			highest = 7
		} else if optHighest == nil {
			lowest = int(optLowest.IntValue())
			highest = lowest + 14
		} else if optLowest == nil {
			highest = int(optHighest.IntValue())
			lowest = highest - 14
		} else {
			lowest = int(optLowest.IntValue())
			highest = int(optHighest.IntValue())
		}

		if highest-lowest > 24 {
			panic("range too large")
		}

		components = makeModifierButtonPad(base.StringValue(), lowest, highest)
	}

	// add instructions
	components = append(components, discordgo.TextDisplay{
		Content: fmt.Sprintf("-# Click or tap to make dice rolls! Results will post to <#%s>.", i.ChannelID),
	})

	err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
			Components: components,
		},
	})

	// only send the tip message if we're not already in a DM
	// if err == nil && !dm {
	// 	MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
	// 		Type: discordgo.InteractionResponseChannelMessageWithSource,
	// 		Data: &discordgo.InteractionResponseData{
	// 			Content: "Sent you a direct message!",
	// 			Flags:   discordgo.MessageFlagsEphemeral,
	// 		},
	// 	})
	// } else
	// if there was an error for the interaction, send a different error response
	if err != nil {
		logger.Error("error sending message", zap.Error(err))
		MeasureInteractionRespond(s.InteractionRespond, i, errRes)
		return
	}
}

// InteractionHealth sends bot state information.
func InteractionHealth(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	logger.Debug("state handler called", zap.String("interaction", i.ID))

	if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:  discordgo.MessageFlagsSuppressNotifications,
			Embeds: makeHealthEmbed(ctx),
		},
	}); err != nil {
		logger.Error("error sending response", zap.Error(err))
	}
}

// InteractionStats sends bot stats.
func InteractionStats(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	logger.Debug("stats handler called", zap.String("interaction", i.ID))

	if !DiceGolem.IsOwner(UserFromInteraction(i)) {
		if err := MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("This command is reserved for bot administrators.")); err != nil {
			logger.Error("error sending response", zap.Error(err))
		}
	} else {
		if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: makeStatsEmbed(ctx),
			},
		}); err != nil {
			logger.Error("error sending response", zap.Error(err))
		}
	}
}

func InteractionClear(ctx context.Context) {
	s, i, _ := FromContext(ctx)

	options := i.ApplicationCommandData().Options
	u := UserFromInteraction(i)
	switch options[0].Name {
	case "recent":
		// clear out recent roll key from the cache
		if DiceGolem.Cache.Redis != nil {
			key := fmt.Sprintf(KeyCacheUserRecentFmt, u.ID)
			DiceGolem.Cache.Redis.Del(ctx, key)
		}
		if err := MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Cleared your cached roll history (if any).")); err != nil {
			logger.Error("error sending response", zap.Error(err))
		}
	case "expressions":
		_ = InteractionExpressionsClear(ctx, u)
		if err := MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Cleared your saved expressions (if any).")); err != nil {
			logger.Error("error sending response", zap.Error(err))
		}
	default:
		if err := MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Sorry, an invalid subcommand was received!")); err != nil {
			logger.Error("subcommand error", zap.Error(err))
		}
	}
}

func InteractionPreferences(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	user := UserFromInteraction(i)
	options := i.ApplicationCommandData().Options
	switch options[0].Name {
	case "recent":
		option := mustGetOptionByName(options, "enabled")
		if option.BoolValue() {
			// reset to default 'enabled' setting
			UserUnsetPreference(user, SettingNoRecent)
		} else {
			UserSetPreference(user, SettingNoRecent)
			DiceGolem.Cache.Redis.Del(ctx, fmt.Sprintf(KeyCacheUserRecentFmt, user.ID))
		}
	case "output":
		option := mustGetOptionByName(options, "detailed")
		if option.BoolValue() {
			// set default of 'True'
			UserSetPreference(user, SettingDetailed)
		} else {
			UserUnsetPreference(user, SettingDetailed)
		}
	default:
		panic(fmt.Sprintf("unhandled preference: %s", options[0].Name))
	}
	MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("Updated your preference."))
}

func InteractionSettings(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	options := i.ApplicationCommandData().Options
	switch options[0].Name {
	case "forward":
		logger.Debug("updating forwarding settings")
		dest := mustGetOptionByName(options, "to")
		source := getOptionByName(options, "from")
		destChannel := dest.ChannelValue(nil)
		sourceChannel := source.ChannelValue(nil)
		// if in the same guild (sanity check)
		if destChannel.GuildID == i.GuildID {
			if destChannel.ID == sourceChannel.ID {
				// clear setting if setting to same channel
				DiceGolem.Cache.Redis.Del(ctx, fmt.Sprintf("setting:%s:%s:%s", i.GuildID, sourceChannel.ID, SettingForward))
			} else {
				GuildChannelSetNamedSetting(destChannel.GuildID, sourceChannel.ID, SettingForward, destChannel.ID)
				if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Embeds: []*discordgo.MessageEmbed{
							{
								Color:       0x7493f3,
								Description: fmt.Sprintf("Roll forwarding configured: %s → %s", sourceChannel.Mention(), destChannel.Mention()),
							},
						},
					},
				}); err != nil {
					logger.Error("response error", zap.Error(err))
				}
			}
		} else {
			// unreachable
			panic(ErrUnexpectedError)
		}
	default:
		panic(fmt.Sprintf("unhandled setting: %s", options[0].Name))
	}
}

func InteractionDebug(ctx context.Context) {
	s, i, _ := FromContext(ctx)

	MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{
					Content: "Testing!",
				},
			},
		},
	})
}

// InteractionExpressionsClear drops a user's saved expressions from the backend
// store, if they exit.
func InteractionExpressionsClear(ctx context.Context, u *discordgo.User) error {
	// TODO: check Del() return code (int => number of deleted keys)
	if DiceGolem.Cache.Redis != nil {
		key := fmt.Sprintf(KeyCacheUserGlobalExpressionsFmt, u.ID)
		DiceGolem.Cache.Redis.Del(ctx, key)
	}
	return nil
}

func InteractionGolemancy(ctx context.Context) {
	s, i, _ := FromContext(ctx)
	if !DiceGolem.IsOwner(UserFromInteraction(i)) {
		if err := MeasureInteractionRespond(s.InteractionRespond, i, newEphemeralResponse("This command is reserved for bot administrators.")); err != nil {
			logger.Error("error sending response", zap.Error(err))
		}
		return
	}
	data := i.ApplicationCommandData()
	logger.Debug("command data", zap.Any("data", data))
	group := data.Options[0]
	switch group.Name {
	case "restart":
		subcommand := group.Options[0]
		switch subcommand.Name {
		case "shard":
			id := getOptionByName(subcommand.Options, "id").IntValue()
			if session := DiceGolem.Sessions[id]; session != nil {
				MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				})
				now := time.Now()
				defer metrics.MeasureSince([]string{"session", "restart"}, now)
				if err := closeSession(session); err != nil {
					logger.Error("error closing shard", zap.Error(err), zap.Int("shard", s.ShardID))
				}
				if err := openSession(session); err != nil {
					logger.Error("error opening shard", zap.Error(err), zap.Int("shard", s.ShardID))
				}
				if _, err := s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
					Embeds: Ptr([]*discordgo.MessageEmbed{
						{
							Description: fmt.Sprintf("Session %d restarted (%s)!", s.ShardID, humanfmt.Sprintf("%s", time.Since(now))),
						},
					}),
				},
				); err != nil {
					logger.Error("error responding to restart", zap.Error(err))
				}
			}
		}
	default:
		panic(fmt.Errorf("unhandled golemancy subcommand: %s", group.Name))
	}
}

func MeasureInteractionRespond(fn func(*discordgo.Interaction, *discordgo.InteractionResponse, ...discordgo.RequestOption) error, i *discordgo.Interaction, r *discordgo.InteractionResponse) error {
	defer metrics.MeasureSince([]string{"interaction", "send"}, time.Now())
	return fn(i, r)
}

// UserFromInteraction returns the User that spawned the supplied interaction.
// If the interaction was sent in a guild the user will be drawn from the
// interaction Member, otherwise it will be the User.
func UserFromInteraction(i *discordgo.Interaction) (user *discordgo.User) {
	if i == nil {
		return nil
	}

	if i.Member != nil {
		return i.Member.User
	}
	return i.User
}

// UserFromInteraction returns the User that sent the supplied message.
// If the message was sent in a guild the user will be drawn from the
// interaction Member, otherwise it will be the Author.
func UserFromMessage(m *discordgo.Message) (user *discordgo.User) {
	if m == nil {
		return nil
	}

	if m.Member != nil && m.Member.User != nil {
		return m.Member.User
	}

	return m.Author
}

// isInteractionPublic returns whether an interaction would be shown to users
// other than the sender.
func isInteractionPublic(i *discordgo.Interaction) bool {
	fmt.Printf("%#v\n", i)
	// if the command wasn't /roll or Roll Message, it's automatically private
	if !contains([]string{"roll", "Roll Message"}, i.ApplicationCommandData().Name) {
		return false
	}

	// no guild member data, therefore already from a DM
	if i.Member == nil {
		return false
	}

	// determine if the message was sent secretly or privately
	options := i.ApplicationCommandData().Options
	if optEphemeral := getOptionByName(options, "secret"); optEphemeral != nil && optEphemeral.BoolValue() {
		return false
	}
	if optPrivate := getOptionByName(options, "private"); optPrivate != nil && optPrivate.BoolValue() {
		return false
	}
	return true
}

// ExpressionsClearInteraction drops a user's saved expressions from the backend
// store, if they exit.
func ExpressionsClearInteraction(ctx context.Context, u *discordgo.User) error {
	// TODO: check Del() return code (int => number of deleted keys)
	if DiceGolem.Cache.Redis != nil {
		key := fmt.Sprintf(KeyCacheUserGlobalExpressionsFmt, u.ID)
		DiceGolem.Cache.Redis.Del(ctx, key)
	}
	return nil
}

func makeSaveExpressionModal(seed *NamedRollInput) *discordgo.InteractionResponse {
	if seed == nil {
		seed = new(NamedRollInput)
	}
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "modal_save",
			Title:    "Save Expression",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "name",
							Label:       "Roll name",
							Style:       discordgo.TextInputShort,
							Value:       seed.Name,
							Placeholder: "Cast Sleep",
							MaxLength:   32,
							MinLength:   1,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "expression",
							Label:       "Expression to roll",
							Style:       discordgo.TextInputShort,
							Value:       seed.Expression,
							Placeholder: "5d8+1",
							Required:    true,
							MaxLength:   100,
							MinLength:   1,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "label",
							Label:       "Label for result",
							Style:       discordgo.TextInputShort,
							Value:       seed.Label,
							Placeholder: "affected HP",
							MaxLength:   32,
							MinLength:   1,
						},
					},
				},
			},
		},
	}
}

func makeEditExpressionsModal(csv string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "modal_import",
			Title:    "Edit Expressions",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "csv",
							Label:       "Expression data (CSV format)",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "expression,name,label\n1d20+4,Check Perception,Perception\n4d6k3,Stat roll,\n",
							Value:       csv,
							Required:    true,
							MaxLength:   2000,
						},
					},
				},
			},
		},
	}
}

func makeMultiRollModal() *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "modal_bulk",
			Title:    "Bulk Roll",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "expression",
							Label:       "Multiline expression",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "1d20 + 4 # to hit\n3d8 + 4 # damage",
							Required:    true,
							MaxLength:   180,
						},
					},
				},
			},
		},
	}
}
