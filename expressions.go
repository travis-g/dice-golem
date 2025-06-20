package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// pray this is never used in a roll or label
var delim = '|'

type NamedRollInput struct {
	Expression string `json:"e" mapstructure:"expression" csv:"expression"`
	Name       string `json:"n,omitempty" mapstructure:"name,omitempty" csv:"name"`
	Label      string `json:"l,omitempty" mapstructure:"label,omitempty" csv:"label"`
}

type RollSlice []*NamedRollInput

// Validate validates that a NamedRollInput's fields are valid.
func (i *NamedRollInput) Validate() error {
	if i.Expression == "" {
		return errors.New("empty expression")
	}
	if len(i.Expression) > 100 {
		return errors.New("expression too long")
	}
	if i.Name != "" && len(i.Name) > 32 {
		return errors.New("name too long")
	}
	if i.Label != "" && len(i.Label) > 32 {
		return errors.New("label too long")
	}
	return nil
}

func (i *NamedRollInput) Clean() {
	i.Expression = strings.TrimSpace(i.Expression)
	i.Name = strings.TrimSpace(i.Name)
	i.Label = strings.TrimSpace(i.Label)
}

// String returns a human-readable string like "Name (Expression, Label)".
func (i *NamedRollInput) String() string {
	if i.Name != "" && i.Label != "" {
		return fmt.Sprintf("%s (%s, %s)", i.Name, i.Expression, i.Label)
	}
	if i.Name != "" && i.Label == "" {
		return fmt.Sprintf("%s (%s)", i.Name, i.Expression)
	}
	if i.Label != "" && i.Name == "" {
		return fmt.Sprintf("%s, %s", i.Expression, i.Label)
	}
	return i.Expression
}

// RollableString returns a rollable expression.
func (i *NamedRollInput) RollableString() string {
	var b strings.Builder
	b.WriteString(i.Expression)
	if i.Label != "" {
		b.WriteString(" # ")
		b.WriteString(i.Label)
	}
	return b.String()
}

// okForAutocomplete returns whether the roll input can be safely used as a
// discordgo.ApplicationCommandOptionChoice based on Discord's property limits.
// If not ok an error indicating validation reason is returned.
func (i *NamedRollInput) okForAutocomplete(ctx context.Context) (ok bool, _ error) {
	if len(i.RollableString()) > 100 {
		return false, errors.New("combined expression and label exceed Discord limit")
	}
	if len(i.String()) > 100 {
		return false, errors.New("combined roll data length exceeds Discord limit")
	}
	return true, nil
}

// ID returns the unique ID for the roll used to distinguish it.
func (i *NamedRollInput) ID() string {
	if i.Name != "" {
		return i.Name
	}
	return i.Serialize()
}

func (i *NamedRollInput) Serialize() string {
	if i == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(i.Expression)
	b.WriteRune(delim)
	if i.Label != "" {
		b.WriteString(i.Label)
	}
	b.WriteRune(delim)
	if i.Name != "" {
		b.WriteString(i.Name)
	}
	return b.String()
}

func (i *NamedRollInput) Deserialize(serial string) {
	if i == nil {
		i = new(NamedRollInput)
	}
	if serial == "" {
		return
	}
	parts := strings.Split(serial, string(delim))
	i.Expression = parts[0]
	if len(parts) > 1 {
		i.Label = parts[1]
	}
	if len(parts) > 2 {
		i.Name = parts[2]
	}
}

// Clone returns a deep copy of the NamedRollInput.
func (i *NamedRollInput) Clone() *NamedRollInput {
	if i == nil {
		return nil
	}
	return &NamedRollInput{
		Expression: i.Expression,
		Name:       i.Name,
		Label:      i.Label,
	}
}

func NamedRollInputsFromMap(m map[string]string) RollSlice {
	rolls := []*NamedRollInput{}
	for _, val := range m {
		roll := new(NamedRollInput)
		if err := json.Unmarshal([]byte(val), roll); err == nil {
			rolls = append(rolls, roll)
		}
	}
	return rolls
}

func SetNamedRoll(u *discordgo.User, gid string, r *NamedRollInput) (_ error) {
	ctx := context.TODO()
	if DiceGolem.Cache.Redis == nil {
		return ErrNoRedisClient
	}
	if ok, err := r.okForAutocomplete(ctx); !ok {
		return err
	}

	b, err := json.Marshal(r)
	if err != nil {
		logger.Error("error marshalling roll", zap.Error(err))
	}

	key := fmt.Sprintf(KeyCacheUserGlobalExpressionsFmt, u.ID)
	if _, err = DiceGolem.Cache.Redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		defer DiceGolem.Cache.Remove(key)
		pipe.HSet(ctx, key, r.ID(), string(b))
		// re-set TTL for all saved data
		pipe.Expire(ctx, key, DiceGolem.DataTTL)
		return nil
	}); err != nil {
		logger.Error("error saving roll", zap.Error(err))
	}

	return err
}

func GetNamedRolls(u *discordgo.User, gid string) (RollSlice, error) {
	ctx := context.TODO()
	key := fmt.Sprintf(KeyCacheUserGlobalExpressionsFmt, u.ID)

	data := DiceGolem.Cache.HGetAll(ctx, key)
	return NamedRollInputsFromMap(data), nil
}

func FilterNamedRollInputs(input string, targets RollSlice) RollSlice {
	options := make([]string, len(targets))
	stringMap := make(map[string]*NamedRollInput)
	for i, option := range targets {
		options[i] = option.String()
		stringMap[option.String()] = option
	}

	matches := fuzzy.RankFindNormalizedFold(input, options)
	sort.Sort(matches)
	options = TargetsFromRanks(matches)

	// build the choices list from filtered opts
	choices := make([]*NamedRollInput, len(options))
	for i, option := range options {
		entry := stringMap[option]
		choices[i] = entry
	}
	return choices
}
