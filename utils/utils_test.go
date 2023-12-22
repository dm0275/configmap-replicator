package utils

import "testing"

func TestListContains(t *testing.T) {
	type args struct {
		s []string
		e string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "list contains item",
			args: args{
				s: []string{"red", "blue", "green"},
				e: "green",
			},
			want: true,
		},
		{
			name: "list does not contain item",
			args: args{
				s: []string{"red", "blue", "green"},
				e: "purple",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ListContains(tt.args.s, tt.args.e); got != tt.want {
				t.Errorf("ListContains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSlicesOverlap(t *testing.T) {
	type args struct {
		slice1 []string
		slice2 []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "slices do not overlap",
			args: args{
				slice1: []string{"one", "two"},
				slice2: []string{"three"},
			},
			want: false,
		},
		{
			name: "slices partially overlap",
			args: args{
				slice1: []string{"one", "two"},
				slice2: []string{"one"},
			},
			want: true,
		},
		{
			name: "slices overlap",
			args: args{
				slice1: []string{"one", "two"},
				slice2: []string{"one", "two"},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SlicesOverlap(tt.args.slice1, tt.args.slice2); got != tt.want {
				t.Errorf("SlicesOverlap() = %v, want %v", got, tt.want)
			}
		})
	}
}
