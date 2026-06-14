package server

import "testing"

func TestIsTruthyEnv(t *testing.T) {
	truthy := []string{"1", "true", "TRUE", "Yes", "on", "  on  "}
	for _, v := range truthy {
		if !isTruthyEnv(v) {
			t.Errorf("isTruthyEnv(%q) = false, want true", v)
		}
	}

	falsy := []string{"", "0", "false", "no", "off", "maybe", "2"}
	for _, v := range falsy {
		if isTruthyEnv(v) {
			t.Errorf("isTruthyEnv(%q) = true, want false", v)
		}
	}
}
