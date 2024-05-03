package main

import (
	"reflect"
	"testing"
)

func Test_makeModifierRange(t *testing.T) {
	type args struct {
		lowest  int
		highest int
	}
	tests := []struct {
		name string
		args args
		want []int
	}{
		{
			name: "single",
			args: args{-1, -1},
			want: []int{-1},
		},
		{
			name: "several",
			args: args{-1, 1},
			want: []int{-1, 0, 1},
		},
		{
			name: "several-1",
			args: args{5, 15},
			want: []int{5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := makeModifierRange(tt.args.lowest, tt.args.highest); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeModifierRange() = %v, want %v", got, tt.want)
			}
		})
	}
}
