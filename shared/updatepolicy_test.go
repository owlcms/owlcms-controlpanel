package shared

import "testing"

func TestShouldProposeUpdate(t *testing.T) {
	cases := []struct {
		installed string
		candidate string
		want      bool
	}{
		{"68.0.2", "68.0.3", true},
		{"68.0.2", "69.0.0-rc01", false},
		{"68.0.2", "69.0.0-alpha00", false},
		{"68.0.0-rc16", "69.0.0-alpha00", false},
		{"68.0.0-beta03", "69.0.0-alpha00", false},
		{"68.0.0-rc16", "68.0.2-rc01", true},
		{"68.0.0-beta03", "68.0.0-rc01", true},
		{"68.0.0-rc16", "68.0.0", true},
		{"69.0.0-alpha00", "69.0.0-alpha01", true},
		{"69.0.0-alpha00", "69.0.0-beta01", true},
		{"69.0.0-alpha00", "69.0.0", true},
		{"68.0.0-SNAPSHOT", "69.0.0-alpha00", false},
		{"firmata-v1.2.0-rc01", "firmata-v1.3.0-alpha01", false},
		{"firmata-v1.2.0-alpha01", "firmata-v1.3.0-alpha01", true},
		{"v3.3.5", "v3.3.6-alpha02", false},
	}
	for _, c := range cases {
		if got := ShouldProposeUpdate(c.installed, c.candidate); got != c.want {
			t.Errorf("ShouldProposeUpdate(%q, %q) = %v, want %v", c.installed, c.candidate, got, c.want)
		}
	}
}

func TestIsAlpha(t *testing.T) {
	for _, v := range []string{"69.0.0-alpha00", "v3.3.6-alpha02", "firmata-v1.3.0-alpha01", "69.0.0-ALPHA01"} {
		if !IsAlpha(v) {
			t.Errorf("IsAlpha(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"69.0.0", "69.0.0-beta01", "69.0.0-rc01", "69.0.0-SNAPSHOT"} {
		if IsAlpha(v) {
			t.Errorf("IsAlpha(%q) = true, want false", v)
		}
	}
}
