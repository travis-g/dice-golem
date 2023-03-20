package main

import (
	"reflect"
	"testing"
)

func Test_derivedShards(t *testing.T) {
	type args struct {
		clusterIndex  int
		totalClusters int
		numShards     int
	}
	tests := []struct {
		name         string
		args         args
		wantShardIDs []int
		wantErr      bool
	}{
		{
			name: "unclustered",
			args: args{
				0, 1, 6,
			},
			wantShardIDs: []int{0, 1, 2, 3, 4, 5},
			wantErr:      false,
		},
		{
			name: "cluster 0,2",
			args: args{
				0, 2, 6,
			},
			wantShardIDs: []int{0, 2, 4},
			wantErr:      false,
		},
		{
			name: "cluster 1,3",
			args: args{
				1, 3, 6,
			},
			wantShardIDs: []int{1, 4},
			wantErr:      false,
		},
		{
			name: "invalid",
			args: args{
				0, 2, 7,
			},
			wantShardIDs: nil,
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotShardIDs, err := deriveClusterShards(tt.args.clusterIndex, tt.args.totalClusters, tt.args.numShards)
			if (err != nil) != tt.wantErr {
				t.Errorf("derivedShards() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotShardIDs, tt.wantShardIDs) {
				t.Errorf("derivedShards() = %v, want %v", gotShardIDs, tt.wantShardIDs)
			}
		})
	}
}
