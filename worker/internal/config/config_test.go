package config

import (
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDefaults(t *testing.T) {
	c, err := Load(nil, envOf(nil), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	want := &Config{
		Queues:       []string{"default"},
		Labels:       map[string]string{},
		Concurrency:  4,
		DataDir:      DefaultDataDir,
		DrainTimeout: 30 * time.Minute,
		LogRetention: 720 * time.Hour,
		GitImage:     DefaultGitImage,
	}
	if diff := cmp.Diff(want, c); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestFlagOverridesEnv(t *testing.T) {
	env := envOf(map[string]string{
		"LUTE_SERVER":        "from-env:50051",
		"LUTE_NAME":          "env-name",
		"LUTE_QUEUES":        "build, deploy",
		"LUTE_LABELS":        "os=linux, gpu = 2",
		"LUTE_CONCURRENCY":   "2",
		"LUTE_REQUIRE_MOUNT": "1",
		"LUTE_JOB_MEMORY":    "2g",
		"LUTE_JOB_CPUS":      "1.5",
	})
	c, err := Load([]string{"--server", "from-flag:50051", "--concurrency", "8"}, env, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server != "from-flag:50051" {
		t.Errorf("server = %q, want the flag's value", c.Server)
	}
	if c.Concurrency != 8 {
		t.Errorf("concurrency = %d, want the flag's 8", c.Concurrency)
	}
	if c.Name != "env-name" || !c.RequireMount {
		t.Errorf("name %q, require mount %v: env values were lost", c.Name, c.RequireMount)
	}
	if diff := cmp.Diff([]string{"build", "deploy"}, c.Queues); diff != "" {
		t.Errorf("queues (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]string{"os": "linux", "gpu": "2"}, c.Labels); diff != "" {
		t.Errorf("labels (-want +got):\n%s", diff)
	}
	if c.JobMemory != 2<<30 || c.JobNanoCPUs != 1_500_000_000 || !c.LimitsConfigured() {
		t.Errorf("limits: memory %d, nano cpus %d", c.JobMemory, c.JobNanoCPUs)
	}
}

func TestInvalid(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"concurrency": {"LUTE_CONCURRENCY": "0"},
		"labels":      {"LUTE_LABELS": "novalue"},
		"memory":      {"LUTE_JOB_MEMORY": "lots"},
		"cpus":        {"LUTE_JOB_CPUS": "-1"},
		"duration":    {"LUTE_DRAIN_TIMEOUT": "soon"},
		"bool":        {"LUTE_ALLOW_ROOTFUL": "maybe"},
		"queues":      {"LUTE_QUEUES": " , "},
	} {
		if _, err := Load(nil, envOf(env), io.Discard); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := Load([]string{"extra"}, envOf(nil), io.Discard); err == nil {
		t.Error("a stray argument was accepted")
	}
}
