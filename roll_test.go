package main

import (
	"reflect"
	"testing"
)

func TestNewRollInputFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantData *RollInput
	}{
		{
			name:  "full",
			input: "3d6 # swing",
			wantData: &RollInput{
				Expression: "3d6",
				Label:      "swing",
			},
		},
		{
			name:     "empty",
			input:    "",
			wantData: &RollInput{},
		},
		{
			name:  "roll only",
			input: "3d6",
			wantData: &RollInput{
				Expression: "3d6",
				Label:      "",
			},
		},
		{
			name:  "roll, blank comment",
			input: "3d6 # ",
			wantData: &RollInput{
				Expression: "3d6",
				Label:      "",
			},
		},
		{
			name:  "weird spacings",
			input: " 3d6  + 4   ",
			wantData: &RollInput{
				Expression: "3d6  + 4",
				Label:      "",
			},
		},
		{
			name:     "comment only",
			input:    "\\ comment",
			wantData: &RollInput{},
		},
		{
			name:     "comment only; weird spaces",
			input:    " #  comment ",
			wantData: &RollInput{},
		},
		{
			name:     "other comment only; weird spaces",
			input:    " |  comment ",
			wantData: &RollInput{},
		},
		// {
		// 	name:  "leading comment",
		// 	input: "Arcana: d20+2",
		// 	wantData: &RollInput{
		// 		Expression: "d20+2",
		// 		Label:      "Arcana",
		// 	},
		// },
		// {
		// 	name:  "doubled comment",
		// 	input: "Ignored: d20+2 # check",
		// 	wantData: &RollInput{
		// 		Expression: "d20+2",
		// 		Label:      "check",
		// 	},
		// },
		{
			name:  "roll response",
			input: "`3d6`: `(5+1+4)` = **10**",
			wantData: &RollInput{
				Expression: "3d6",
			},
		},
		{
			name:  "roll response",
			input: "`3d6` *fire damage*: `(5+1+4)` = **10**",
			wantData: &RollInput{
				Expression: "3d6",
				// Label:    "fire damage",
			},
		},
		{
			name:  "roll response; bad",
			input: "`3d6 *` *weird*: `(5+1+4)` = **10**",
			wantData: &RollInput{
				Expression: "3d6 *",
				// Label:    "weird",
			},
		},
		{
			name:  "formatted message",
			input: "roll `3d6 +2` bludgeoning",
			wantData: &RollInput{
				Expression: "3d6 +2",
			},
		},
		{
			name:  "formatted message",
			input: "`3d6 +2` damage `bogus`",
			wantData: &RollInput{
				Expression: "3d6 +2",
			},
		},
		{
			name:  "math",
			input: "what's `3+5*8`",
			wantData: &RollInput{
				Expression: "3+5*8",
			},
		},
		{
			name:  "response",
			input: "<!@12345654> rolled `3+5*8`: nonsense",
			wantData: &RollInput{
				Expression: "3+5*8",
			},
		},
		{
			name:  "serialized roll input",
			input: "3d6|fire damage",
			wantData: &RollInput{
				Expression: "3d6",
				Label:      "fire damage",
			},
		},
		{
			name:  "old prefix",
			input: "/roll 3d6",
			wantData: &RollInput{
				Expression: "3d6",
			},
		},
		{
			name:  "empty label",
			input: "3d6 # ",
			wantData: &RollInput{
				Expression: "3d6",
			},
		},
		{
			name:  "label with mention",
			input: "3d6 # @trav#1234 test",
			wantData: &RollInput{
				Expression: "3d6",
				Label:      "@trav#1234 test",
			},
		},
		// {
		// 	name:     "formatted message; broken",
		// 	input:    "roll `3d6 bludgeoning",
		// 	wantData: &RollInput{},
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotData := NewRollInputFromString(tt.input); !reflect.DeepEqual(gotData, tt.wantData) {
				t.Errorf("NewRollInputFromString() = %+v, want %+v", gotData, tt.wantData)
			}
		})
	}
}
