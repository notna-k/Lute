package models

import (
	"strings"
	"testing"
)

func TestValidateWorkerName(t *testing.T) {
	for _, ok := range []string{"build-01", "a", "ci.example.com", "x_y"} {
		if err := ValidateWorkerName(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-lead", "has space", "sl/ash", strings.Repeat("a", 64)} {
		if err := ValidateWorkerName(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestValidateWorkerLabels(t *testing.T) {
	if err := ValidateWorkerLabels(map[string]string{"os": "linux", "gpu.count": "2"}); err != nil {
		t.Error(err)
	}
	if err := ValidateWorkerLabels(map[string]string{"bad key": "x"}); err == nil {
		t.Error("a key with a space was accepted")
	}
	if err := ValidateWorkerLabels(map[string]string{"k": strings.Repeat("v", 256)}); err == nil {
		t.Error("a 256-char value was accepted")
	}
}
