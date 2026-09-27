package version

import "testing"

func TestOlder(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"0.1.0", "0.2.0", true},
		{"0.2.0", "0.2.0", false},
		{"0.2.1", "0.2.0", false},
		{"v0.9.9", "1.0.0", true},
		{"0.10.0", "0.9.0", false},
		{"dev", "0.2.0", false},
		{"0.1.0", "dev", false},
		{"0.1", "0.2.0", false},
	}
	for _, tt := range tests {
		if got := Older(tt.a, tt.b); got != tt.want {
			t.Errorf("Older(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestMinorTag(t *testing.T) {
	for v, want := range map[string]string{"0.2.5": "0.2", "v1.0.0": "1.0", "dev": "latest"} {
		if got := MinorTag(v); got != want {
			t.Errorf("MinorTag(%q) = %q, want %q", v, got, want)
		}
	}
}
