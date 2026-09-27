package runner

import (
	"archive/tar"
	"io"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/google/go-cmp/cmp"
)

func TestShellCommand(t *testing.T) {
	// The script runs under bash where the image has it and sh where it does not, so a
	// job on an alpine-based runtime starts instead of failing at container creation.
	want := []string{"sh", "-c", shellPicker, "lute", "/lute/cmd.sh"}
	got := shellCommand(scriptPath)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestScriptArchive(t *testing.T) {
	r, err := scriptArchive("echo hi\n")
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(r)
	type entry struct {
		Name string
		Mode int64
		Body string
	}
	var got []entry
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(tr)
		got = append(got, entry{h.Name, h.Mode, string(body)})
	}
	want := []entry{{"lute/", 0o755, ""}, {"lute/cmd.sh", 0o755, "echo hi\n"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestValidateGitHubRepo(t *testing.T) {
	for _, ok := range []string{"", "https://github.com/octocat/Hello-World"} {
		if err := validateGitHubRepo(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"http://github.com/a/b", "https://gitlab.com/a/b", "git@github.com:a/b"} {
		if err := validateGitHubRepo(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestLastLine(t *testing.T) {
	if got := lastLine("Cloning...\n7fd1a60b01f91b314f59955a4e4d4e80d8edf11d\n\n"); got != "7fd1a60b01f91b314f59955a4e4d4e80d8edf11d" {
		t.Errorf("got %q", got)
	}
}

func TestDigestOf(t *testing.T) {
	pulled := image.InspectResponse{ID: "sha256:local", RepoDigests: []string{"bash@sha256:abc"}}
	if got := digestOf(pulled); got != "sha256:abc" {
		t.Errorf("pulled image: %q", got)
	}
	built := image.InspectResponse{ID: "sha256:local"}
	if got := digestOf(built); got != "sha256:local" {
		t.Errorf("local image: %q", got)
	}
}
