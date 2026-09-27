// Package state keeps the agent's identity: what core returned from Register.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type State struct {
	WorkerID string `json:"worker_id"`
	Secret   string `json:"secret"`
	Server   string `json:"server"`
}

// Load returns nil and no error when the agent has not registered yet.
func Load(path string) (*State, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is the agent's own data dir
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON (delete it to register again): %w", path, err)
	}
	if s.WorkerID == "" || s.Secret == "" {
		return nil, fmt.Errorf("%s has no worker_id or secret (delete it to register again)", path)
	}
	return &s, nil
}

// Save writes the file atomically with mode 0600: a crash leaves the old file or the new
// one, never half of either.
func Save(path string, s *State) error {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".state-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
