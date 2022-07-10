package main

import (
	"context"
	"strconv"

	"go.uber.org/zap"
)

// excessiveDiceMiddleware checks the expression available in the request
// context to determine if the desired roll is dangerous/will require too many
// dice.
func excessiveDiceMiddleware(next HandlerFunc) HandlerFunc {
	return HandlerFunc(func(ctx context.Context) {
		if excessiveDice(ctx) {
			return
		}
		next(ctx)
	})
}

// excessiveDice predicts whether the context's dice expression would exceed
// the maximum allowed number of dice per roll.
func excessiveDice(ctx context.Context) bool {
	roll, ok := ctx.Value(KeyRollInput).(*RollInput)
	if !ok {
		panic("dice expression missing from context")
	}
	matches := manyDice.FindAllStringSubmatch(roll.Expression, -1)
	count := 0
	for _, ext := range matches {
		num, _ := strconv.Atoi(ext[1])
		count += num
	}
	if count > DiceGolem.MaxDice {
		logger.Debug("too many dice",
			zap.String("expression", roll.Expression),
			zap.Int("count", count),
		)
		return true
	}
	return false
}
