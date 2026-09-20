package setup

import (
	"os"
	"path/filepath"
)

// DaemonLogPath is the file where `setup` redirects the background worker's
// stdout and stderr. `lute-worker logs` reads this path.
func DaemonLogPath() string {
	return filepath.Join(os.TempDir(), "lute-worker.log")
}
