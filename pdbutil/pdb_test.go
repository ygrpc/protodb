package pdbutil

import "testing"

func TestIsZeroValue(t *testing.T) {
	type args struct {
		val interface{}
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "zero int",
			args: args{
				val: 0,
			},
			want: true,
		},
		{
			name: "zero string",
			args: args{
				val: "",
			},
			want: true,
		},
		//dobule 0.0
		{
			name: "zero double",
			args: args{
				val: 0.0,
			},
			want: true,
		},
		//float 0.0
		{
			name: "zero float",
			args: args{
				val: float32(0.0),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsZeroValue(tt.args.val); got != tt.want {
				t.Errorf("IsZeroValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
