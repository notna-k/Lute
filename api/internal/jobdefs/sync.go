package jobdefs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// SyncResult counts what a sync did, for the log line and the panel.
type SyncResult struct {
	Added     int `json:"added"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
	// Detached are definitions no longer in Git, kept because prune is off.
	Detached int `json:"detached"`
	Pruned   int `json:"pruned"`
	// Skipped lists files or documents that failed to parse.
	Skipped []string `json:"skipped"`
}

// Syncer reconciles the job-definitions directory with Postgres. Git is the
// source of truth for every change it makes; the panel's edits stand until the
// file they diverged from changes.
type Syncer struct {
	defs     *repos.JobDefinitionRepository
	settings *repos.SettingRepository
	dir      string
	mu       sync.Mutex
}

func NewSyncer(defs *repos.JobDefinitionRepository, settings *repos.SettingRepository, dir string) *Syncer {
	return &Syncer{defs: defs, settings: settings, dir: strings.TrimSpace(dir)}
}

// Sync runs one pass. A missing or unset directory is a no-op, not an error —
// otherwise every definition would be detached (or pruned) by a typo.
func (s *Syncer) Sync(ctx context.Context) (SyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res := SyncResult{Skipped: []string{}}
	if s.dir == "" {
		slog.Warn("JOB_DEFS_DIR not set, skipping job-definition sync")
		return res, nil
	}
	if info, err := os.Stat(s.dir); err != nil || !info.IsDir() {
		slog.Warn("job-definition dir not readable, skipping sync", "dir", s.dir)
		return res, nil
	}

	files, skipped, err := loadDir(s.dir)
	if err != nil {
		return res, err
	}
	current, err := s.defs.List(ctx)
	if err != nil {
		return res, err
	}
	prune, err := s.settings.GetBool(ctx, models.PruneDefinitions)
	if err != nil {
		return res, err
	}

	plan := reconcile(current, files, prune)
	plan.result.Skipped = skipped
	for _, def := range plan.creates {
		if err := s.defs.Create(ctx, def); err != nil {
			return res, fmt.Errorf("create %s: %w", def.Slug, err)
		}
	}
	for _, def := range plan.updates {
		if err := s.defs.Update(ctx, def); err != nil {
			return res, fmt.Errorf("update %s: %w", def.Slug, err)
		}
	}
	if err := s.defs.DeleteSlugs(ctx, plan.deletes); err != nil {
		return res, fmt.Errorf("prune: %w", err)
	}

	r := plan.result
	slog.Info("synced job definitions", "dir", s.dir, "added", r.Added, "updated", r.Updated,
		"unchanged", r.Unchanged, "detached", r.Detached, "pruned", r.Pruned, "skipped", len(r.Skipped))
	return r, nil
}

// syncPlan is the set of writes one sync needs.
type syncPlan struct {
	creates []*models.JobDefinition
	updates []*models.JobDefinition
	deletes []string
	result  SyncResult
}

// reconcile decides what a sync writes, given what Postgres holds and what Git
// says. It does no I/O so the rules can be tested directly:
//
//   - new in Git                 → created
//   - Git changed since last sync → overwritten, panel edits included
//   - Git unchanged              → left alone, so panel edits survive
//   - gone from Git              → deleted when prune is on, else detached
//     (kept, and flagged in the panel)
func reconcile(current []models.JobDefinition, files []*models.JobDefinition, prune bool) syncPlan {
	var plan syncPlan
	bySlug := make(map[string]*models.JobDefinition, len(current))
	for i := range current {
		bySlug[current[i].Slug] = &current[i]
	}

	seen := make(map[string]bool, len(files))
	for _, f := range files {
		seen[f.Slug] = true
		spec := f.JobSpec
		existing, ok := bySlug[f.Slug]
		if !ok {
			f.GitSpec = &spec
			plan.creates = append(plan.creates, f)
			plan.result.Added++
			continue
		}
		moved := existing.SourcePath != f.SourcePath || existing.SourceCommit != f.SourceCommit
		existing.SourcePath, existing.SourceCommit = f.SourcePath, f.SourceCommit
		if existing.GitSpec != nil && existing.GitSpec.Equal(spec) {
			if moved {
				plan.updates = append(plan.updates, existing)
			}
			plan.result.Unchanged++
			continue
		}
		existing.JobSpec = spec
		existing.GitSpec = &spec
		plan.updates = append(plan.updates, existing)
		plan.result.Updated++
	}

	for i := range current {
		def := &current[i]
		if seen[def.Slug] {
			continue
		}
		switch {
		case prune:
			plan.deletes = append(plan.deletes, def.Slug)
			plan.result.Pruned++
		case def.GitSpec != nil:
			def.GitSpec = nil
			plan.updates = append(plan.updates, def)
			plan.result.Detached++
		}
	}
	return plan
}

// loadDir parses every *.yaml / *.yml under dir. A file may hold several
// definitions as `---`-separated documents, which is what Export produces. A
// file that fails to parse is skipped (and reported), not fatal: one typo must
// not detach every definition. Duplicate slugs keep the first occurrence.
func loadDir(dir string) ([]*models.JobDefinition, []string, error) {
	var (
		out     []*models.JobDefinition
		skipped = []string{}
		seen    = map[string]string{}
	)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if d.IsDir() || (ext != ".yaml" && ext != ".yml") {
			return nil
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			rel = filepath.Base(path)
		}
		rel = filepath.ToSlash(rel)

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		defs, perr := parseDocs(data, rel)
		if perr != nil {
			slog.Warn("skipping job-definition file", "file", rel, "err", perr)
			skipped = append(skipped, fmt.Sprintf("%s: %v", rel, perr))
			return nil
		}
		for _, def := range defs {
			if first, dup := seen[def.Slug]; dup {
				msg := fmt.Sprintf("%s: slug %q already defined in %s", rel, def.Slug, first)
				slog.Warn("skipping duplicate job definition", "file", rel, "slug", def.Slug, "first", first)
				skipped = append(skipped, msg)
				continue
			}
			seen[def.Slug] = rel
			out = append(out, def)
		}
		return nil
	})
	sort.Strings(skipped)
	return out, skipped, err
}
