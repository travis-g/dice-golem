package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/bwmarrin/discordgo"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"go.uber.org/zap"
)

type Serialer interface {
	Serialize() string
	Deserialize(s string)
	String() string
}

func fuzzyFilterSerials(partial string, recentSerials, savedSerials []string) (matchingSerials []string) {
	serials := append(savedSerials, recentSerials...)
	matches := fuzzy.RankFindNormalizedFold(partial, serials)
	sort.Sort(matches)
	matchingSerials = TargetsFromRanks(matches)
	return
}

func RollSliceFromSerials(serials []string) RollSlice {
	rolls := make([]*RollInput, len(serials))
	for i, serial := range serials {
		var ri = new(RollInput)
		ri.Deserialize(serial)
		rolls[i] = ri
	}
	return rolls
}

func SuggestExpression(ctx context.Context) {
	s, i, _ := FromContext(ctx)

	data := i.ApplicationCommandData()
	user := UserFromInteraction(i)

	recents, err := CachedSerials(user)
	logger.Debug("cached serials", zap.Any("serials", recents))
	if err != nil {
		logger.Error("cache error", zap.Error(err))
	}

	// fuzzy-filtered stored rolls
	var slice RollSlice
	if input := getOptionByName(data.Options, "expression").StringValue(); input == "" {
		slice = RollSliceFromSerials(recents)
	} else {
		// include current input first
		serials := []string{input}
		// only sort by similarity if the user's entered something. by default
		// the ranking should be by recency
		serials = append(serials, fuzzyFilterSerials(input, recents, []string{})...)
		slice = RollSliceFromSerials(serials)
	}

	choices := DistinctChoices(ChoicesFromRollSlice(slice))

	logger.Debug("choices", zap.Any("data", choices))

	_ = MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
}

func SuggestName(ctx context.Context) {
	s, i, _ := FromContext(ctx)

	data := i.ApplicationCommandData()
	u := UserFromInteraction(i)

	choices := []*discordgo.ApplicationCommandOptionChoice{}
	var input string
	switch {
	case getOptionByName(data.Options, "name") != nil:
		input = getOptionByName(data.Options, "name").StringValue()
	case getOptionByName(data.Options, "expression") != nil:
		input = getOptionByName(data.Options, "expression").StringValue()
	default:
		panic("unreachable code")
	}

	rolls := CachedNamedRolls(fmt.Sprintf(CacheKeyUserRollsFormat, u.ID))

	switch {
	case data.Name == "expressions" && data.Options[0].Name == "unsave":
		options := make([]string, len(rolls))
		stringMap := make(map[string]*NamedRollInput)
		for i, option := range rolls {
			options[i] = option.String()
			stringMap[option.String()] = option
		}

		if input != "" {
			matches := fuzzy.RankFindNormalizedFold(input, options)
			sort.Sort(matches)
			options = TargetsFromRanks(matches)
		}

		// build the choices list from filtered opts
		for _, option := range options {
			entry := stringMap[option]
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  entry.String(),
				Value: entry.ID(),
			})
		}
	case data.Name == "expressions" && data.Options[0].Name == "save":
		// only suggest existing expression names to overwrite or current input
		options := []string{}
		nameMap := make(map[string]*NamedRollInput)
		for _, option := range rolls {
			if option.Name != "" {
				options = append(options, option.Name)
				nameMap[option.Name] = option
			}
		}

		// if we have input, add the input as a choice, then rank filter the
		// other options
		if input != "" {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  input,
				Value: input,
			})

			matches := fuzzy.RankFindNormalizedFold(input, options)
			sort.Sort(matches)
			options = TargetsFromRanks(matches)
		}

		for _, option := range options {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  nameMap[option].String(),
				Value: nameMap[option].ID(),
			})
		}
	default:
	}

	logger.Debug("name choices", zap.Any("data", choices))

	if err := MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	}); err != nil {
		logger.Error("autocomplete", zap.Error(err))
	}
}

func SuggestLabel(ctx context.Context) {
	s, i, _ := FromContext(ctx)

	data := i.ApplicationCommandData()
	user := UserFromInteraction(i)
	rolls, err := CachedRolls(user)
	logger.Debug("cached rolls", zap.Any("rolls", rolls))
	if err != nil {
		logger.Error("cache error", zap.Error(err))
	}

	// TODO: include current input when dedup options
	options := DistinctRollLabels(rolls)
	input := getOptionByName(data.Options, "label").StringValue()
	if input != "" {
		// only sort by similarity if the user's entered something. by default
		// the ranking should be by recency
		options = append([]string{input}, options...)
		matches := fuzzy.RankFindNormalizedFold(input, options)
		sort.Sort(matches)
		options = TargetsFromRanks(matches)
	}
	choices := DistinctChoices(ChoicesFromStrings(options))
	logger.Debug("choices", zap.Any("data", choices))

	if err = MeasureInteractionRespond(s.InteractionRespond, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	}); err != nil {
		logger.Error("autocomplete", zap.Error(err))
	}
}

func ChoicesFromRollSlice(rolls RollSlice) []*discordgo.ApplicationCommandOptionChoice {
	if len(rolls) == 0 {
		return []*discordgo.ApplicationCommandOptionChoice{}
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, len(rolls))
	for i, roll := range rolls {
		choice := &discordgo.ApplicationCommandOptionChoice{
			// FIXME: wait until parser improvements are made
			// Name:  strings.TrimSpace(fmt.Sprintf("%s %s", roll.Expression, roll.Label)),
			// Value: roll.Serialize(),
			Name:  roll.Expression,
			Value: roll.Expression,
		}
		choices[i] = choice
	}
	return choices
}

func ChoicesFromStrings(slice []string) []*discordgo.ApplicationCommandOptionChoice {
	if len(slice) == 0 {
		return []*discordgo.ApplicationCommandOptionChoice{}
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, len(slice))
	for i, value := range slice {
		choice := &discordgo.ApplicationCommandOptionChoice{
			Value: value,
			Name:  value,
		}
		choices[i] = choice
	}
	return choices
}

// DistinctChoices deduplicates a set of option choices by the choices' Names.
func DistinctChoices(choices []*discordgo.ApplicationCommandOptionChoice) (list []*discordgo.ApplicationCommandOptionChoice) {
	uniques := make(map[string]bool)
	for _, choice := range choices {
		if _, found := uniques[choice.Name]; !found {
			uniques[choice.Name] = true
			list = append(list, choice)
		}
	}
	return
}

func DistinctRollExpressions(rolls []RollInput) (list []string) {
	expressions := make(map[string]bool)
	for _, roll := range rolls {
		if roll.Expression == "" {
			continue
		}
		if _, found := expressions[roll.Expression]; !found {
			expressions[roll.Expression] = true
			list = append(list, roll.Expression)
		}
	}
	return list
}

func DistinctRollLabels(rolls []RollInput) (list []string) {
	labels := make(map[string]bool)
	for _, roll := range rolls {
		if roll.Label == "" {
			continue
		}
		if _, found := labels[roll.Label]; !found {
			labels[roll.Label] = true
			list = append(list, roll.Label)
		}
	}
	return list
}

func DistinctExpressionNames(rolls []*NamedRollInput) (list []string) {
	names := make(map[string]bool)
	for _, roll := range rolls {
		if roll.Name == "" {
			continue
		}
		if _, found := names[roll.Name]; !found {
			names[roll.Name] = true
			list = append(list, roll.Name)
		}
	}
	return
}

func TargetsFromRanks(ranks fuzzy.Ranks) []string {
	var targets = make([]string, len(ranks))
	for i, rank := range ranks {
		targets[i] = rank.Target
	}
	return targets
}
