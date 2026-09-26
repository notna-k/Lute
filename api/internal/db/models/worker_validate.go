package models

import (
	"fmt"
	"regexp"
)

var (
	labelKeyRe   = regexp.MustCompile(`^[a-zA-Z0-9_\-.]{1,63}$`)
	workerNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_\-.]{0,62}$`)
)

// ValidateWorkerName allows host-name-like names: they appear in logs and the Docker command.
func ValidateWorkerName(name string) error {
	if !workerNameRe.MatchString(name) {
		return fmt.Errorf("invalid worker name %q: 1-63 chars of letters, digits, '.', '_' or '-', starting with a letter or digit", name)
	}
	return nil
}

func ValidateWorkerLabels(labels map[string]string) error {
	if len(labels) > 32 {
		return fmt.Errorf("too many labels: max 32, got %d", len(labels))
	}
	for k, v := range labels {
		if !labelKeyRe.MatchString(k) {
			return fmt.Errorf("invalid label key %q: must be 1-63 chars, alphanumeric, underscore, hyphen or dot", k)
		}
		if len(v) > 255 {
			return fmt.Errorf("label value for key %q exceeds 255 characters", k)
		}
	}
	return nil
}
