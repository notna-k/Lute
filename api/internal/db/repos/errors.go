package repos

import (
	"errors"

	"gorm.io/gorm"
)

// ErrNotFound is returned when a single-row lookup finds no matching record.
var ErrNotFound = errors.New("not found")

// ErrJobNotRunning means the result is stale: the reaper already gave up on this attempt.
var ErrJobNotRunning = errors.New("job is not running")

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
