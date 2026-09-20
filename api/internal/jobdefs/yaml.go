package jobdefs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/lute/api/internal/db/models"
)

// yamlJob is the on-disk shape of a job definition. It is both what the sync
// reads and what Export writes, so a panel edit can be pasted back into Git
// and read as the same spec.
type yamlJob struct {
	Slug        string            `yaml:"slug,omitempty"`
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Queue       string            `yaml:"queue,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Runtime     string            `yaml:"runtime"`
	Command     string            `yaml:"command"`
	Source      *yamlSource       `yaml:"source,omitempty"`
	Parameters  []yamlParam       `yaml:"parameters,omitempty"`
}

type yamlSource struct {
	Repo   string `yaml:"repo,omitempty"`
	Commit string `yaml:"commit,omitempty"`
}

type yamlParam struct {
	Name        string       `yaml:"name"`
	Type        string       `yaml:"type"`
	Label       string       `yaml:"label,omitempty"`
	EnvVar      string       `yaml:"env,omitempty"`
	Description string       `yaml:"description,omitempty"`
	Required    bool         `yaml:"required,omitempty"`
	Default     any          `yaml:"default,omitempty"`
	Options     []yamlOption `yaml:"options,omitempty"`
	SecretRef   string       `yaml:"secretRef,omitempty"`
}

type yamlOption struct {
	Value string `yaml:"value"`
	Label string `yaml:"label,omitempty"`
	Hint  string `yaml:"hint,omitempty"`
	Tone  string `yaml:"tone,omitempty"`
}

// parseDocs reads every `---`-separated definition in one file. Any invalid
// document fails the whole file, so a half-applied file never happens.
func parseDocs(data []byte, path string) ([]*models.JobDefinition, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var out []*models.JobDefinition
	for i := 1; ; i++ {
		var y yamlJob
		err := dec.Decode(&y)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		if reflect.ValueOf(y).IsZero() {
			continue // blank document, e.g. a trailing `---`
		}
		def, err := y.toDefinition(path)
		if err != nil {
			return nil, fmt.Errorf("document %d: %w", i, err)
		}
		out = append(out, def)
	}
}

func (y yamlJob) toDefinition(path string) (*models.JobDefinition, error) {
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

	params := make([]models.ParameterField, 0, len(y.Parameters))
	for _, p := range y.Parameters {
		if strings.TrimSpace(p.Name) == "" {
			return nil, fmt.Errorf("parameter with no name")
		}
		if !KnownTypes[p.Type] {
			return nil, fmt.Errorf("parameter %q has unknown type %q", p.Name, p.Type)
		}
		if (p.Type == "select" || p.Type == "multiselect") && len(p.Options) == 0 {
			return nil, fmt.Errorf("parameter %q of type %s needs options", p.Name, p.Type)
		}
		var opts []models.ParameterOption
		for _, o := range p.Options {
			opts = append(opts, models.ParameterOption(o))
		}
		envVar := p.EnvVar
		if envVar == "" {
			envVar = strings.ToUpper(slugRe.ReplaceAllString(strings.ToLower(p.Name), "_"))
		}
		params = append(params, models.ParameterField{
			Name:        p.Name,
			Type:        p.Type,
			Label:       p.Label,
			EnvVar:      envVar,
			Description: p.Description,
			Required:    p.Required,
			Default:     p.Default,
			Options:     opts,
			SecretRef:   p.SecretRef,
		})
	}

	queue := y.Queue
	if queue == "" {
		queue = "default"
	}
	def := &models.JobDefinition{
		Slug: slug,
		JobSpec: models.JobSpec{
			Name:          y.Name,
			Description:   y.Description,
			Queue:         queue,
			LabelSelector: y.Labels,
			Runtime:       y.Runtime,
			Command:       y.Command,
			Parameters:    params,
		},
		SourcePath: path,
	}
	if y.Source != nil {
		def.SourceRepo = y.Source.Repo
		def.SourceCommit = y.Source.Commit
	}
	return def, nil
}

// toYAML is the inverse of toDefinition: the file that would produce def.
func toYAML(def *models.JobDefinition) yamlJob {
	y := yamlJob{
		Name:        def.Name,
		Description: def.Description,
		Queue:       def.Queue,
		Labels:      def.LabelSelector,
		Runtime:     def.Runtime,
		Command:     def.Command,
	}
	// A slug the name already implies is noise; one that differs (a renamed
	// definition keeps its slug so its builds stay attached) must be kept.
	if def.Slug != slugify(def.Name) {
		y.Slug = def.Slug
	}
	if def.SourceRepo != "" || def.SourceCommit != "" {
		y.Source = &yamlSource{Repo: def.SourceRepo, Commit: def.SourceCommit}
	}
	for _, p := range def.Parameters {
		yp := yamlParam{
			Name:        p.Name,
			Type:        p.Type,
			Label:       p.Label,
			EnvVar:      p.EnvVar,
			Description: p.Description,
			Required:    p.Required,
			Default:     p.Default,
			SecretRef:   p.SecretRef,
		}
		for _, o := range p.Options {
			yp.Options = append(yp.Options, yamlOption(o))
		}
		y.Parameters = append(y.Parameters, yp)
	}
	return y
}

// filePath is where def lives in Git, or where it should go if it never has.
func filePath(def *models.JobDefinition) string {
	if def.SourcePath != "" {
		return def.SourcePath
	}
	return def.Slug + ".yaml"
}

// Export renders definitions as one multi-document YAML stream. Each document
// is headed by the file it belongs in, so the output can be split back into
// files — or committed as one, since the sync reads multi-document files.
func Export(defs []models.JobDefinition) ([]byte, error) {
	var buf bytes.Buffer
	for i := range defs {
		body, err := yaml.MarshalWithOptions(toYAML(&defs[i]), yaml.UseLiteralStyleIfMultiline(true))
		if err != nil {
			return nil, fmt.Errorf("export %s: %w", defs[i].Slug, err)
		}
		if i > 0 {
			buf.WriteString("---\n")
		}
		fmt.Fprintf(&buf, "# %s\n", filePath(&defs[i]))
		buf.Write(body)
	}
	return buf.Bytes(), nil
}
