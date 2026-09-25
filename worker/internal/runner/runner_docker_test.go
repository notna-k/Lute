package runner

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestShellCommand(t *testing.T) {
	// The script runs under bash where the image has it and sh where it does not, so a
	// job on an alpine-based runtime starts instead of failing at container creation.
	want := []string{"sh", "-c", shellPicker, "lute", "/workspace/_user_command.sh"}
	got := shellCommand("/workspace/_user_command.sh")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
	if got[0] == "bash" {
		t.Error("the command demands bash outright; alpine images have only sh")
	}
}
