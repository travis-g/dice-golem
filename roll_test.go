package main

import (
	"reflect"
	"testing"
)

func TestNewRollInputFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantData *NamedRollInput
	}{
		{
			name:  "full",
			input: "3d6 # swing",
			wantData: &NamedRollInput{
				Expression: "3d6",
				Label:      "swing",
			},
		},
		{
			name:     "empty",
			input:    "",
			wantData: &NamedRollInput{},
		},
		{
			name:  "roll only",
			input: "3d6",
			wantData: &NamedRollInput{
				Expression: "3d6",
				Label:      "",
			},
		},
		{
			name:  "roll, blank comment",
			input: "3d6 # ",
			wantData: &NamedRollInput{
				Expression: "3d6",
				Label:      "",
			},
		},
		{
			name:  "weird spacings",
			input: " 3d6  + 4   ",
			wantData: &NamedRollInput{
				Expression: "3d6  + 4",
				Label:      "",
			},
		},
		{
			name:     "comment only",
			input:    "\\ comment",
			wantData: &NamedRollInput{},
		},
		{
			name:     "comment only; weird spaces",
			input:    " #  comment ",
			wantData: &NamedRollInput{},
		},
		{
			name:     "other comment only; weird spaces",
			input:    " |  comment ",
			wantData: &NamedRollInput{},
		},
		// {
		// 	name:  "leading comment",
		// 	input: "Arcana: d20+2",
		// 	wantData: &NamedRollInput{
		// 		Expression: "d20+2",
		// 		Label:      "Arcana",
		// 	},
		// },
		// {
		// 	name:  "doubled comment",
		// 	input: "Ignored: d20+2 # check",
		// 	wantData: &NamedRollInput{
		// 		Expression: "d20+2",
		// 		Label:      "check",
		// 	},
		// },
		{
			name:  "roll response",
			input: "`3d6`: `(5+1+4)` = **10**",
			wantData: &NamedRollInput{
				Expression: "3d6",
			},
		},
		{
			name:  "roll response",
			input: "`3d6` _fire damage_: `(5+1+4)` = **10**",
			wantData: &NamedRollInput{
				Expression: "3d6",
				Label:      "fire damage",
			},
		},
		{
			name:  "roll response; bad",
			input: "`3d6 *` _weird_: `(5+1+4)` = **10**",
			wantData: &NamedRollInput{
				Expression: "3d6 *",
				Label:      "weird",
			},
		},
		{
			name:  "formatted message",
			input: "roll `3d6 +2` bludgeoning",
			wantData: &NamedRollInput{
				Expression: "3d6 +2",
			},
		},
		{
			name:  "formatted message",
			input: "`3d6 +2` damage `bogus`",
			wantData: &NamedRollInput{
				Expression: "3d6 +2",
			},
		},
		{
			name:  "math",
			input: "what's `3+5*8`",
			wantData: &NamedRollInput{
				Expression: "3+5*8",
			},
		},
		{
			name:  "response",
			input: "<!@12345654> rolled `3+5*8`: nonsense",
			wantData: &NamedRollInput{
				Expression: "3+5*8",
			},
		},
		{
			name:  "serialized roll input",
			input: "3d6|fire damage",
			wantData: &NamedRollInput{
				Expression: "3d6",
				Label:      "fire damage",
			},
		},
		{
			name:  "old prefix",
			input: "/roll 3d6",
			wantData: &NamedRollInput{
				Expression: "3d6",
			},
		},
		{
			name:  "empty label",
			input: "3d6 # ",
			wantData: &NamedRollInput{
				Expression: "3d6",
			},
		},
		{
			name:  "label with mention",
			input: "3d6 # @trav#1234 test",
			wantData: &NamedRollInput{
				Expression: "3d6",
				Label:      "@trav#1234 test",
			},
		},
		// {
		// 	name:     "formatted message; broken",
		// 	input:    "roll `3d6 bludgeoning",
		// 	wantData: &NamedRollInput{},
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

func Test_splitMultirollString(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want []string
	}{
		{
			name: "basic",
			arg:  "foo ; bar",
			want: []string{"foo ", " bar"},
		},
		{
			name: "single",
			arg:  "foo",
			want: []string{"foo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitMultirollString(tt.arg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitMultirollString() = %v, want %v", got, tt.want)
			}
		})
	}
}
