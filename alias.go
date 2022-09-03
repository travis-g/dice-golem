package main

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
)

type NamedRollInput struct {
	Name string
	RollInput
}

func (i *NamedRollInput) String() string {
	if i.Name != i.Expression {
		return fmt.Sprintf("%s (%s)", i.Name, i.Expression)
	}
	return i.Expression
}

const (
	RedisKeyUserSavedFormat = "cache:user:%s:saved"
)

func SavedRolls(u *discordgo.User) (rolls []NamedRollInput, err error) {
	if DiceGolem.Redis == nil {
		return []NamedRollInput{}, ErrNoRedisClient
	}
	defer metrics.MeasureSince([]string{"cache", "saved_rolls"}, time.Now())

	key := fmt.Sprintf(RedisKeyUserSavedFormat, u.ID)
	func() {
		defer metrics.MeasureSince([]string{"redis", "hgetall"}, time.Now())
		data, err := DiceGolem.Redis.HGetAllMap(key).Result()
		if err != nil {
			return
		}
		rolls = NamedRollInputsFromMap(data)
	}()
	return
}

func NamedRollInputsFromMap(m map[string]string) []NamedRollInput {
	rolls := []NamedRollInput{}
	for name, val := range m {
		roll := NamedRollInput{Name: name}
		roll.Deserialize(val)
		rolls = append(rolls, roll)
	}
	return rolls
}
