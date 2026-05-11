package update

import "testing"

func TestIsOutdated(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
	}{
		{"", "v1.0.0", false},
		{"dev", "v1.0.0", false},
		{"v1.0.0", "", false},
		{"v1.0.0", "v1.0.0", false},
		{"1.0.0", "v1.0.0", false},
		{"v1.0.0", "1.0.0", false},
		{"v1.0.0", "v1.0.1", true},
		{"v1.0.0", "v2.0.0", true},
	}
	for _, tt := range tests {
		t.Run(tt.current+"_vs_"+tt.latest, func(t *testing.T) {
			if got := IsOutdated(tt.current, tt.latest); got != tt.want {
				t.Fatalf("IsOutdated(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestDisplayVersion(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"dev", "dev"},
		{"v1.2.3", "v1.2.3"},
		{"1.2.3", "v1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := DisplayVersion(tt.in); got != tt.want {
				t.Fatalf("DisplayVersion(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
