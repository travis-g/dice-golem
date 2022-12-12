package main

import "testing"

func Test_truncString(t *testing.T) {
	type args struct {
		s   string
		num int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "pass-through", args: args{s: "testing", num: 10}, want: "testing"},
		{name: "trunc", args: args{s: "testing", num: 4}, want: "test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncString(tt.args.s, tt.args.num); got != tt.want {
				t.Errorf("truncString() = %v, want %v", got, tt.want)
			}
		})
	}
}
