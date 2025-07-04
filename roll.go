package main

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/travis-g/dice/math"
	"go.uber.org/zap"
)

// commentFieldFunc returns whether a rune is a comment character. Used for
// splitting roll inputs that contain comments/labels.
func commentFieldFunc(r rune) bool {
	return r == '\\' || r == '#' || r == delim
}

var (
	mentionRegexp        = regexp.MustCompile(`<@.+?>`)
	multirollSplitRegexp = regexp.MustCompile(`[\n;]`)
)

// NewRollInputFromString returns a new RollInput based off an input string with
// optional comment (i.e. label).
//
// TODO: fuzzy-parse rolls from inputs, ex. "roll 3d6 + 4 damage" => "3d6 + 4"
func NewRollInputFromString(input string) *NamedRollInput {
	// strip any mentions and Discord-specific tag things
	input = mentionRegexp.ReplaceAllString(input, "")
	input = strings.TrimPrefix(input, "/roll")
	// strip any leading/trailing whitespace
	input = strings.TrimSpace(input)
	// if empty input or notation starts with a comment, short-circuit
	if input == "" || commentFieldFunc([]rune(input)[0]) {
		return new(NamedRollInput)
	}

	data := new(NamedRollInput)
	code := input

	// check if there's a roll notation or expression
	reCode := regexp.MustCompile("`(.+?)`")
	matches := reCode.FindStringSubmatch(code)
	if len(matches) > 0 {
		data = NewRollInputFromString(matches[1])
		code = matches[1]
	}

	reLabel := regexp.MustCompile(`_(.+?)_`)
	matches = reLabel.FindStringSubmatch(input)
	if len(matches) > 0 {
		data.Label = matches[1]
	}

	// split at label/comment
	parts := strings.FieldsFunc(code, commentFieldFunc)
	data.Expression = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		// remove everything prior to the first split loc carefully. first rune
		// after cutting off the expression will be a comment char
		data.Label = strings.TrimSpace(strings.TrimPrefix(input, parts[0])[1:])
	}

	return data
}

// NewRollInputFromMessage parses and returns a RollInput from the content
// of a Discord message.
func NewRollInputFromMessage(m *discordgo.Message) (data *NamedRollInput) {
	return NewRollInputFromString(m.Content)
}

// EvaluateRollInputWithContext takes a RollInput and executes it, returning a
// Response and any errors encountered.
func EvaluateRollInputWithContext(ctx context.Context, rollInput *NamedRollInput) (res *Response, err error) {
	defer recover()
	s, i, m := FromContext(ctx)
	if s == nil || (i == nil && m == nil) {
		panic("context data missing")
	}

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
	logger.Debug("evaluated roll", zap.Any("response", res))

	go trackRollFromContext(ctx)

	res.Rolled = res.ExpressionResult.Rolled
	res.Result = strconv.FormatFloat(res.ExpressionResult.Result, 'f', -1, 64)
	return
}

// evaluateRoll executes the given roll string and emits metrics.
func evaluateRoll(ctx context.Context, roll string) (*math.ExpressionResult, error) {
	defer metrics.MeasureSince([]string{"roll", "evaluate"}, time.Now())
	ctx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	return math.EvaluateExpression(ctx, roll)
}

func splitMultirollString(s string) []string {
	return multirollSplitRegexp.Split(s, -1)
}
