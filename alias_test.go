package main

import (
	"reflect"
	"testing"
)

func TestNamedRollInputsFromMap(t *testing.T) {
	type args struct {
		m map[string]string
	}
	tests := []struct {
		name string
		args args
		want []NamedRollInput
	}{
		{
			name: "basic",
			args: args{
				m: map[string]string{
					"Test": "1d20|testing",
				},
			},
			want: []NamedRollInput{
				{
					Name: "Test",
					RollInput: RollInput{
						Expression: "1d20",
						Label:      "testing",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NamedRollInputsFromMap(tt.args.m); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NamedRollInputsFromMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
