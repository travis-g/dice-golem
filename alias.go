package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
	redis "gopkg.in/redis.v3"
)

type NamedRollInput struct {
	Expression string `json:"e"`
	Name       string `json:"n,omitempty"`
	Label      string `json:"l,omitempty"`
}

func (i *NamedRollInput) Validate() bool {
	if i.Expression == "" || len(i.Expression) > 128 {
		return false
	}
	if i.Name != "" && len(i.Name) > 32 {
		return false
	}
	if i.Label != "" && len(i.Label) > 32 {
		return false
	}
	return true
}

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

// RollString returns a rollable expression.
func (i *NamedRollInput) RollString() string {
	var b strings.Builder
	b.WriteString(i.Expression)
	if i.Label != "" {
		b.WriteString(" # ")
		b.WriteString(i.Label)
	}
	return b.String()
}

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
		_ = i
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

func NamedRollInputsFromMap(m map[string]string) []*NamedRollInput {
	rolls := []*NamedRollInput{}
	for _, val := range m {
		roll := new(NamedRollInput)
		if err := json.Unmarshal([]byte(val), roll); err == nil {
			rolls = append(rolls, roll)
		}
	}
	return rolls
}

func SetNamedRoll(u *discordgo.User, gid string, r *NamedRollInput) (err error) {
	if DiceGolem.Redis == nil {
		return ErrNoRedisClient
	}
	key := fmt.Sprintf(CacheKeyUserRollsFormat, u.ID)
	b, _ := json.Marshal(r)

	if _, err = DiceGolem.Redis.Pipelined(func(pipe *redis.Pipeline) error {
		pipe.HSet(key, r.ID(), string(b))
		// re-set TTL for all saved data
		pipe.Expire(key, DiceGolem.DataTTL)
		return nil
	}); err != nil {
		logger.Error("error saving roll", zap.Error(err))
	}

	return
}

func GetNamedRoll(u *discordgo.User, gid string, id string) (r *NamedRollInput, err error) {
	if DiceGolem.Redis == nil {
		return nil, ErrNoRedisClient
	}
	key := fmt.Sprintf(CacheKeyUserRollsFormat, u.ID)

	var data string
	if data, err = DiceGolem.Redis.HGet(key, id).Result(); err != nil {
		return nil, err
	}

	_ = json.Unmarshal([]byte(data), r)
	return
}

func GetNamedRolls(u *discordgo.User, gid string) ([]*NamedRollInput, error) {
	if DiceGolem.Redis == nil {
		return nil, ErrNoRedisClient
	}
	key := fmt.Sprintf(CacheKeyUserRollsFormat, u.ID)

	data, err := DiceGolem.Redis.HGetAllMap(key).Result()
	if err != nil {
		return nil, err
	}

	return NamedRollInputsFromMap(data), err
}
