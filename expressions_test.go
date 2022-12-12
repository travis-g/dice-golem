package main

import (
	"testing"
)

func TestNamedRollInput_String(t *testing.T) {
	type fields struct {
		Name       string
		Expression string
		Label      string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "full",
			fields: fields{
				Name:       "Inflict Wounds",
				Expression: "3d10",
				Label:      "necrotic dmg",
			},
			want: "Inflict Wounds (3d10, necrotic dmg)",
		},
		{
			name: "expression",
			fields: fields{
				Expression: "3d10",
			},
			want: "3d10",
		},
		{
			name: "named",
			fields: fields{
				Name:       "test",
				Expression: "3d10",
			},
			want: "test (3d10)",
		},
		{
			name: "label only",
			fields: fields{
				Expression: "3d10",
				Label:      "label",
			},
			want: "3d10, label",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &NamedRollInput{
				Name:       tt.fields.Name,
				Expression: tt.fields.Expression,
				Label:      tt.fields.Label,
			}
			if got := i.String(); got != tt.want {
				t.Errorf("NamedRollInput.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNamedRollInput_Serialize(t *testing.T) {
	type fields struct {
		Name       string
		Expression string
		Label      string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "full",
			fields: fields{
				Name:       "Inflict Wounds",
				Expression: "3d10",
				Label:      "necrotic dmg",
			},
			want: "3d10|necrotic dmg|Inflict Wounds",
		},
		{
			name: "expression only",
			fields: fields{
				Expression: "3d10",
			},
			want: "3d10||",
		},
		{
			name: "named",
			fields: fields{
				Name:       "test",
				Expression: "3d10",
			},
			want: "3d10||test",
		},
		{
			name: "label only",
			fields: fields{
				Expression: "3d10",
				Label:      "label",
			},
			want: "3d10|label|",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &NamedRollInput{
				Name:       tt.fields.Name,
				Expression: tt.fields.Expression,
				Label:      tt.fields.Label,
			}
			if got := i.Serialize(); got != tt.want {
				t.Errorf("NamedRollInput.Serialize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNamedRollInput_ID(t *testing.T) {
	type fields struct {
		Expression string
		Name       string
		Label      string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{name: "full",
			fields: fields{
				Name:       "Inflict Wounds",
				Expression: "3d10",
				Label:      "necrotic dmg",
			},
			want: "Inflict Wounds",
		},
		{
			name: "expression only",
			fields: fields{
				Expression: "3d10",
			},
			want: "3d10||",
		},
		{
			name: "named",
			fields: fields{
				Name:       "test",
				Expression: "3d10",
			},
			want: "test",
		},
		{
			name: "label only",
			fields: fields{
				Expression: "3d10",
				Label:      "label",
			},
			want: "3d10|label|",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &NamedRollInput{
				Expression: tt.fields.Expression,
				Name:       tt.fields.Name,
				Label:      tt.fields.Label,
			}
			if got := i.ID(); got != tt.want {
				t.Errorf("NamedRollInput.ID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNamedRollInput_RollString(t *testing.T) {
	type fields struct {
		Expression string
		Name       string
		Label      string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{name: "full",
			fields: fields{
				Name:       "Inflict Wounds",
				Expression: "3d10",
				Label:      "necrotic dmg",
			},
			want: "3d10 # necrotic dmg",
		},
		{
			name: "expression only",
			fields: fields{
				Expression: "3d10",
			},
			want: "3d10",
		},
		{
			name: "label only",
			fields: fields{
				Expression: "3d10",
				Label:      "label",
			},
			want: "3d10 # label",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &NamedRollInput{
				Expression: tt.fields.Expression,
				Name:       tt.fields.Name,
				Label:      tt.fields.Label,
			}
			if got := i.RollableString(); got != tt.want {
				t.Errorf("NamedRollInput.RollString() = %v, want %v", got, tt.want)
			}
		})
	}
}
