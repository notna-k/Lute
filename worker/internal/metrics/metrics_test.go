package metrics

import "testing"

func TestCollectReportsCanonicalKeys(t *testing.T) {
	m := Collect()
	for _, key := range []string{"cpu_load", "mem_usage_mb", "disk_used_gb", "disk_total_gb"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing %q in %v", key, m)
		}
	}
	if m["mem_usage_mb"] <= 0 {
		t.Errorf("mem_usage_mb = %v, want > 0", m["mem_usage_mb"])
	}
}
