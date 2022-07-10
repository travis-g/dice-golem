package main

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
)

// Constant fmt string formats for settings keys.
const (
	SettingsKeyUserSettingsFormat = "settings:user:%s"
)

// TODO: redo settings management with binary
type SettingName string

const (
	NoRecent SettingName = "norecent"
	Detailed SettingName = "detailed"
)

func (s SettingName) String() string {
	return string(s)
}

func Set(u *discordgo.User, s SettingName) {
	DiceGolem.Redis.SAdd(fmt.Sprintf(SettingsKeyUserSettingsFormat, u.ID), s.String())
}

func Unset(u *discordgo.User, s SettingName) {
	DiceGolem.Redis.SRem(fmt.Sprintf(SettingsKeyUserSettingsFormat, u.ID), s.String())
}

func IsSet(u *discordgo.User, s SettingName) bool {
	defer metrics.MeasureSince([]string{"redis", "sismember"}, time.Now())
	return DiceGolem.Redis.SIsMember(fmt.Sprintf(SettingsKeyUserSettingsFormat, u.ID), s.String()).Val()
}
