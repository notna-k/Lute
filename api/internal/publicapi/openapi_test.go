package publicapi

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-yaml"

	"github.com/lute/api/internal/config"
	luteGrpc "github.com/lute/api/internal/grpc"
	"github.com/lute/api/internal/worker"
)

const publicBase = "/api/public/v1"

// The docs site renders api/openapi.yaml, so it must describe exactly the routes mounted here.
func TestOpenAPIMatchesPublicRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	wh := worker.NewWorkerHandler(&config.Config{}, nil, nil, nil, &luteGrpc.Server{})
	SetupPublicRoutes(r.Group(publicBase), nil, Handlers{
		Runs: NewRunsHandler(nil),
		Jobs: NewJobsHandler(nil, nil),
		Meta: NewMetaHandler(nil, nil),
	}, wh)

	var mounted []string
	for _, rt := range r.Routes() {
		mounted = append(mounted, rt.Method+" "+rt.Path)
	}
	documented := specOperations(t, "../../openapi.yaml")

	for _, op := range documented {
		if !slices.Contains(mounted, op) {
			t.Errorf("openapi.yaml documents %s, which is not mounted", op)
		}
	}
	for _, op := range mounted {
		if !slices.Contains(documented, op) {
			t.Errorf("%s is mounted but missing from openapi.yaml", op)
		}
	}
}

// An unquoted comma inside a YAML flow mapping ends the value and turns the rest into a key
// with no value, so a description silently loses its second half. Every value must be set.
func TestOpenAPIHasNoEmptyValues(t *testing.T) {
	data, err := os.ReadFile("../../openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec any
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	var walk func(node any, path string)
	walk = func(node any, path string) {
		switch n := node.(type) {
		case map[string]any:
			for k, v := range n {
				if v == nil {
					t.Errorf("%s.%s has no value; quote the string it was cut from", path, k)
				}
				walk(v, path+"."+k)
			}
		case []any:
			for i, v := range n {
				walk(v, fmt.Sprintf("%s[%d]", path, i))
			}
		}
	}
	walk(spec, "")
}

var pathParam = regexp.MustCompile(`\{([^}]+)\}`)

// specOperations lists "METHOD /api/public/v1/path" per operation, with {param} written gin's way.
func specOperations(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	var ops []string
	for p, item := range spec.Paths {
		route := publicBase + pathParam.ReplaceAllString(p, ":$1")
		for method := range item {
			if method == "parameters" {
				continue
			}
			ops = append(ops, strings.ToUpper(method)+" "+route)
		}
	}
	if len(ops) == 0 {
		t.Fatal("openapi.yaml has no operations")
	}
	return ops
}
