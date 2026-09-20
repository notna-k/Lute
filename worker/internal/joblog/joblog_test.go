package joblog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTail_basic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "job-x.log")
	if err := os.WriteFile(p, []byte("a\nb\nc"), 0644); err != nil {
		t.Fatal(err)
	}
	r := ReadTail(p, 10, 0)
	if r.Err != "" {
		t.Fatal(r.Err)
	}
	if len(r.Lines) != 3 || r.Lines[0] != "a" || r.Lines[2] != "c" {
		t.Fatalf("lines=%q", r.Lines)
	}
	if r.HasMore {
		t.Fatal("unexpected has_more at BOF")
	}
	if r.NextAnchor != 0 {
		t.Fatalf("next anchor=%d", r.NextAnchor)
	}
}

func TestReadTail_lastN(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "job-x.log")
	if err := os.WriteFile(p, []byte("l1\nl2\nl3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r := ReadTail(p, 2, 0)
	if len(r.Lines) != 2 || r.Lines[0] != "l2" || r.Lines[1] != "l3" {
		t.Fatalf("lines=%q", r.Lines)
	}
	if !r.HasMore {
		t.Fatal("expected has_more")
	}
}

func TestReadTail_olderChunk(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "job-x.log")
	content := "l1\nl2\nl3\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	first := ReadTail(p, 2, 0)
	if first.NextAnchor <= 0 {
		t.Fatalf("next anchor=%d", first.NextAnchor)
	}
	second := ReadTail(p, 2, first.NextAnchor)
	if len(second.Lines) != 1 || second.Lines[0] != "l1" {
		t.Fatalf("lines=%q", second.Lines)
	}
}

func TestReadHead_forward(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "job-x.log")
	if err := os.WriteFile(p, []byte("a\nb\nc"), 0644); err != nil {
		t.Fatal(err)
	}
	r := ReadHead(p, 2, 0)
	if r.Err != "" {
		t.Fatal(r.Err)
	}
	if len(r.Lines) != 2 || r.Lines[0] != "a" || r.Lines[1] != "b" {
		t.Fatalf("lines=%q", r.Lines)
	}
	if !r.HasMore {
		t.Fatal("expected has_more")
	}
	r2 := ReadHead(p, 10, r.NextAnchor)
	if len(r2.Lines) != 1 || r2.Lines[0] != "c" {
		t.Fatalf("lines=%q", r2.Lines)
	}
}

func TestReadTail_noTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "job-x.log")
	if err := os.WriteFile(p, []byte("only"), 0644); err != nil {
		t.Fatal(err)
	}
	r := ReadTail(p, 5, 0)
	if len(r.Lines) != 1 || r.Lines[0] != "only" {
		t.Fatalf("lines=%q", r.Lines)
	}
}
