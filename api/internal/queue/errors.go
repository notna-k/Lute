package queue

import "github.com/lute/api/internal/db/repos"

// ErrNotFound is returned when GetJob misses a persisted slot.
var ErrNotFound = repos.ErrNotFound
