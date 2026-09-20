package jobdefs

import (
	"archive/zip"
	"bytes"
	"testing"
	"time"

	"github.com/lute/api/internal/db/models"
)

func def(slug, path, command string) models.JobDefinition {
	return models.JobDefinition{
		Slug:       slug,
		SourcePath: path,
		JobSpec: models.JobSpec{
			Name: slug, Queue: "default", Runtime: "alpine", Command: command,
		},
	}
}

// One file per job, at the path it has in Git — that is what makes the zip
// unpackable straight over the job-definitions repo.
func TestExportSplitOneFilePerJob(t *testing.T) {
	files, err := ExportSplit([]models.JobDefinition{
		def("a", "nightly/a.yaml", "echo a"),
		def("b", "", "echo b"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if files[0].Path != "nightly/a.yaml" || files[1].Path != "b.yaml" {
		t.Fatalf("got paths %q %q", files[0].Path, files[1].Path)
	}
	for _, f := range files {
		back, err := parseDocs(f.Body, f.Path)
		if err != nil || len(back) != 1 {
			t.Fatalf("%s did not round-trip: %v\n%s", f.Path, err, f.Body)
		}
	}
}

// Two definitions from one multi-document file keep sharing it: splitting by
// path must never let one job's file overwrite another's.
func TestExportSplitKeepsSharedFileTogether(t *testing.T) {
	files, err := ExportSplit([]models.JobDefinition{
		def("a", "all.yaml", "echo a"),
		def("b", "all.yaml", "echo b"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "all.yaml" {
		t.Fatalf("got %d files: %+v", len(files), files)
	}
	back, err := parseDocs(files[0].Body, "all.yaml")
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, files[0].Body)
	}
	if len(back) != 2 || back[0].Slug != "a" || back[1].Slug != "b" {
		t.Fatalf("got %d documents: %+v", len(back), back)
	}
}

// A source path from Git is trusted no further than the archive: nothing may
// unpack outside the directory it is extracted into.
func TestExportSplitKeepsPathsInsideArchive(t *testing.T) {
	files, err := ExportSplit([]models.JobDefinition{
		def("a", "../../etc/a.yaml", "echo a"),
		def("b", "/etc/b.yaml", "echo b"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if files[0].Path != "a.yaml" || files[1].Path != "b.yaml" {
		t.Fatalf("got paths %q %q", files[0].Path, files[1].Path)
	}
}

func TestWriteZipHasAnEntryPerFile(t *testing.T) {
	files := []ExportFile{
		{Path: "nightly/a.yaml", Body: []byte("name: a\n")},
		{Path: "b.yaml", Body: []byte("name: b\n")},
	}
	var buf bytes.Buffer
	if err := writeZip(&buf, files, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.File) != len(files) {
		t.Fatalf("got %d entries, want %d", len(r.File), len(files))
	}
	for i, entry := range r.File {
		if entry.Name != files[i].Path {
			t.Errorf("entry %d = %q, want %q", i, entry.Name, files[i].Path)
		}
		rc, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body := new(bytes.Buffer)
		if _, err := body.ReadFrom(rc); err != nil {
			t.Fatal(err)
		}
		_ = rc.Close()
		if !bytes.Equal(body.Bytes(), files[i].Body) {
			t.Errorf("entry %d body = %q, want %q", i, body.Bytes(), files[i].Body)
		}
	}
}
