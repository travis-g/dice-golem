package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/travis-g/dice/math"
	"go.uber.org/zap"
)

type RollInput struct {
	Expression string `json:"expression"`
	Label      string `json:"label,omitempty"`
}

func (ri *RollInput) String() string {
	if ri.Label == "" {
		return ri.Expression
	}
	return strings.Join([]string{ri.Expression, ri.Label}, " # ")
}

// commentFieldFunc returns whether a rune is a comment character. Used for
// splitting roll inputs that contain comments/labels.
func commentFieldFunc(r rune) bool {
	return r == '\\' || r == '#' || r == delim
}

var mentionRegexp = regexp.MustCompile(`<.+?>`)

// NewRollInputFromString returns a new RollInput based off an input string with
// optional comment (i.e. label).
//
// TODO: fuzzy-parse rolls from inputs, ex. "roll 3d6 + 4 damage" => "3d6 + 4"
func NewRollInputFromString(input string) *RollInput {
	// strip any mentions and Discord-specific tag things
	input = mentionRegexp.ReplaceAllString(input, "")
	input = strings.TrimPrefix(input, "/roll")
	// strip any leading/trailing whitespace
	input = strings.TrimSpace(input)
	// if empty input or notation starts with a comment, short-circuit
	if input == "" || commentFieldFunc([]rune(input)[0]) {
		return &RollInput{}
	}

	// check if there's a roll notation or expression
	reCode := regexp.MustCompile("`(.+?)`")
	matches := reCode.FindStringSubmatch(input)
	if len(matches) > 0 {
		input = matches[1]
	}

	// split at label/comment
	parts := strings.FieldsFunc(input, commentFieldFunc)
	data := &RollInput{
		Expression: strings.TrimSpace(parts[0]),
	}
	if len(parts) > 1 {
		data.Label = strings.TrimSpace(parts[1])
	}

	return data
}

// NewRollInputFromMessage parses and returns a RollInput from the content
// of a Discord message.
func NewRollInputFromMessage(m *discordgo.Message) (data *RollInput) {
	return NewRollInputFromString(m.Content)
}

// EvaluateRollInputWithContext takes a RollInput and executes it, returning a
// Response and any errors encountered.
func EvaluateRollInputWithContext(ctx context.Context, rollInput *RollInput) (res *Response, err error) {
	defer recover()
	s, i, m := FromContext(ctx)
	if s == nil || (i == nil && m == nil) {
		panic(errors.New("context data missing"))
	}

	logger.Debug("evaluating", zap.Any("input", rollInput))
	res = &Response{
		Expression: rollInput.Expression,
		Label:      rollInput.Label,
	}

	var (
		cid string
		id  string
	)
	switch {
	case i != nil:
		id = i.ID
		cid = i.ChannelID
	case m != nil:
		id = m.ID
		cid = m.ChannelID
	}

	logger.Info("rolling",
		zap.String("expression", res.Expression),
		zap.String("label", res.Label),
		zap.String("channel", cid),
		zap.String("id", id),
		zap.Int("shard", s.ShardID),
	)

	res.ExpressionResult, err = evaluateRoll(ctx, res.Expression)

	if err != nil {
		logger.Error("evaluation error",
			zap.String("expression", res.Expression),
			zap.Error(err),
		)

		if strings.Contains(err.Error(), "transition token types") {
			err = ErrTokenTransition
		}
		return
	}

	go trackRollFromContext(ctx)

	// Log the result if debugging
	logger.Debug("rolled",
		zap.Float64("result", res.ExpressionResult.Result),
	)

	res.Rolled = res.ExpressionResult.Rolled
	res.Result = strconv.FormatFloat(res.ExpressionResult.Result, 'f', -1, 64)
	return
}

// evaluateRoll executes the given roll string and emits metrics.
func evaluateRoll(ctx context.Context, roll string) (res *math.ExpressionResult, err error) {
	defer metrics.MeasureSince([]string{"roll", "evaluate"}, time.Now())
	res, err = math.EvaluateExpression(ctx, roll)
	return
}

// cacheRollInput caches the roll from an interaction if an interaction was
// sent successfully.
func cacheRollInput(s *discordgo.Session, i *discordgo.Interaction, roll *RollInput) {
	if responseID, err := GetInteractionResponse(s, i); err == nil {
		DiceGolem.Cache.SetWithTTL(fmt.Sprintf(CacheKeyMessageDataFormat, responseID), roll.Serialize(), DiceGolem.CacheTTL)
	}
}
