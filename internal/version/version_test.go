package version

import "testing"

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "same with v prefix", a: "v0.2.0", b: "0.2.0", want: 0},
		{name: "older patch", a: "0.2.0", b: "0.2.1", want: -1},
		{name: "newer minor", a: "0.3.0", b: "0.2.9", want: 1},
		{name: "dev is older than release", a: "dev", b: "0.2.0", want: -1},
		{name: "empty is older", a: "", b: "0.2.0", want: -1},
		{name: "prerelease is older", a: "0.2.0-rc1", b: "0.2.0", want: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compare(tt.a, tt.b)
			switch {
			case tt.want < 0 && got >= 0:
				t.Fatalf("Compare(%q, %q) = %d, want < 0", tt.a, tt.b, got)
			case tt.want == 0 && got != 0:
				t.Fatalf("Compare(%q, %q) = %d, want 0", tt.a, tt.b, got)
			case tt.want > 0 && got <= 0:
				t.Fatalf("Compare(%q, %q) = %d, want > 0", tt.a, tt.b, got)
			}
		})
	}
}
