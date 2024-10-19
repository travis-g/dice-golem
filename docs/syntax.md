---
publish: false
---

## How to Roll Dice

:memo:

|    `expression`     | Description                                                                                                                                 |
| :-----------------: | ------------------------------------------------------------------------------------------------------------------------------------------- |
|       `d20+2`       | Roll a D20 and add 2 to the result. Basic math operators like `-`, `+`, `*`, `/`, `**` (exponent) and `%` (modulo/remainder) are supported. |
|        `4dF`        | Roll 4 Fudge/Fate dice.                                                                                                                     |
|       `3d6d1`       | Roll three D6s and drop the lowest one. You can keep highest dice with `kh`, drop the highest with `dh`, and keep the lowest with `kl`.     |
|      `2d20kl1`      | Simulate disadvantage by keeping the lowest result out of two D20s.                                                                         |
|      `2d20r1`       | Roll two D20s, re-rolling any 1s. You can also re-roll dice based on comparisons, ex. `2d20r<3` to re-roll all results of 3 or less.        |
|      `d20ro1`       | Roll a D20 and re-roll it only one time if the first result was a 1. Comparison checks work here too, like `10d8ro>4`!                      |
|       `8d6s`        | Roll eight D6s and sort the results in ascending order. To sort results in descending order use `sd`: `8d6sd`.                              |
|   `2d6 + 1d4 + 3`   | Combine dice groups and math together in a single request.                                                                                  |
| `3d6 # Fire damage` | Add an inline label for an expression after a `#` or `\`. The label will be included in the response text.                                  |

### Rolling Secretly

You can use the <span class="mention">/secret</span> and <span class="mention">/private</span> commands [...]

<!-- ### Inline Labels -->

### Named Expressions

### Math Expressions

If you'd like to do a math calculation [...]

## Modifiers

### Rerolling

### Drop/Keep

### Sorting

### Exploding Dice

### Targets

### Critical Success/Failure
