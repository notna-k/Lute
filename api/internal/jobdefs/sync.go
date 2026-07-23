package jobdefs

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
)

// yamlJob is the on-disk shape of a job definition. Git is the source of truth
// (PRODUCT.md); Core syncs these files into Postgres for serving.
type yamlJob struct {
	Slug        string            `yaml:"slug"`
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Queue       string            `yaml:"queue"`
	Labels      map[string]string `yaml:"labels"`
	Runtime     string            `yaml:"runtime"`
	Command     string            `yaml:"command"`
	Source      struct {
		Repo   string `yaml:"repo"`
		Commit string `yaml:"commit"`
	} `yaml:"source"`
	Parameters []yamlParam `yaml:"parameters"`
}

type yamlParam struct {
	Name        string        `yaml:"name"`
	Type        string        `yaml:"type"`
	Label       string        `yaml:"label"`
	Env         string        `yaml:"env"`
	Description string        `yaml:"description"`
	Required    bool          `yaml:"required"`
	Default     any           `yaml:"default"`
	Options     []yamlOption  `yaml:"options"`
	SecretRef   string        `yaml:"secretRef"`
}

type yamlOption struct {
	Value string `yaml:"value"`
	Label string `yaml:"label"`
	Hint  string `yaml:"hint"`
	Tone  string `yaml:"tone"`
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// Sync loads every *.yaml / *.yml under dir, upserts each as a JobDefinition,
// and removes definitions no longer present in the source. It is safe to call
// repeatedly. A missing or empty dir is a no-op (logged, not an error).
func Sync(ctx context.Context, repo *repos.JobDefinitionRepository, dir string) (int, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		log.Println("jobdefs: JOB_DEFS_DIR not set — skipping job-definition sync")
		return 0, nil
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		log.Printf("jobdefs: source dir %q not readable — skipping sync", dir)
		return 0, nil
	}

	var seen []string
	count := 0
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		def, perr := parseFile(path, dir)
		if perr != nil {
			log.Printf("jobdefs: skipping %s: %v", path, perr)
			return nil
		}
		if err := repo.Upsert(ctx, def); err != nil {
			return fmt.Errorf("upsert %s: %w", def.Slug, err)
		}
		seen = append(seen, def.Slug)
		count++
		return nil
	})
	if walkErr != nil {
		return count, walkErr
	}

	if err := repo.DeleteMissing(ctx, seen); err != nil {
		return count, fmt.Errorf("prune removed definitions: %w", err)
	}
	log.Printf("jobdefs: synced %d definition(s) from %s", count, dir)
	return count, nil
}

func parseFile(path, root string) (*models.JobDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var y yamlJob
	if err := yaml.Unmarshal(data, &y); err != nil {
		return nil, err
	}
	if strings.TrimSpace(y.Name) == "" {
		return nil, fmt.Errorf("missing name")
	}
	slug := y.Slug
	if slug == "" {
		slug = slugify(y.Name)
	}
	if slug == "" {
		return nil, fmt.Errorf("could not derive slug")
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = filepath.Base(path)
	}

	params := make([]models.ParameterField, 0, len(y.Parameters))
	for _, p := range y.Parameters {
		opts := make([]models.ParameterOption, 0, len(p.Options))
		for _, o := range p.Options {
			opts = append(opts, models.ParameterOption{Value: o.Value, Label: o.Label, Hint: o.Hint, Tone: o.Tone})
		}
		env := p.Env
		if env == "" {
			env = strings.ToUpper(slugRe.ReplaceAllString(strings.ToLower(p.Name), "_"))
		}
		params = append(params, models.ParameterField{
			Name:        p.Name,
			Type:        p.Type,
			Label:       p.Label,
			EnvVar:      env,
			Description: p.Description,
			Required:    p.Required,
			Default:     p.Default,
			Options:     opts,
			SecretRef:   p.SecretRef,
		})
	}

	return &models.JobDefinition{
		Slug:          slug,
		Name:          y.Name,
		Description:   y.Description,
		Queue:         y.Queue,
		LabelSelector: y.Labels,
		Runtime:       y.Runtime,
		Command:       y.Command,
		SourceRepo:    y.Source.Repo,
		SourcePath:    filepath.ToSlash(rel),
		SourceCommit:  y.Source.Commit,
		Parameters:    params,
	}, nil
}
