package queue

import "errors"

// ErrNotFound is returned when a job id does not exist in the queue store.
var ErrNotFound = errors.New("job not found")
