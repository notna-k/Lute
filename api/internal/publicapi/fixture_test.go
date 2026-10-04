package publicapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	luteGrpc "github.com/lute/api/internal/grpc"
	"github.com/lute/api/internal/queue"
	"github.com/lute/api/internal/runs"
	"github.com/lute/api/internal/testutil/pgtest"
	"github.com/lute/api/internal/worker"
	pb "github.com/lute/proto"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// testUserHeader stands in for the panel's JWT: the panel routes trust it as user_id.
const testUserHeader = "X-Test-User"

// noAgents stands in for the agents' side of gRPC; everything under it is real.
type noAgents struct{ dispatched []string }

func (g *noAgents) DispatchQueue(_ context.Context, q string) { g.dispatched = append(g.dispatched, q) }

func (g *noAgents) RequestJobLog(context.Context, string, *pb.JobLogRequest) (*pb.JobLogResponse, error) {
	return &pb.JobLogResponse{}, nil
}

// fixture is core's HTTP surface for keys and the public API on a fresh database.
type fixture struct {
	t      *testing.T
	router *gin.Engine
	gw     *noAgents
	db     *gorm.DB

	users *repos.UserRepository
	defs  *repos.JobDefinitionRepository
	execs *repos.JobExecutionRepository
	queue *queue.Engine
	runs  *runs.Service
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := connection.Open(context.Background(), pgtest.NewDatabase(t))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	f := &fixture{
		t:     t,
		gw:    &noAgents{},
		db:    db.DB,
		users: repos.NewUserRepository(db.DB),
		defs:  repos.NewJobDefinitionRepository(db.DB),
		execs: repos.NewJobExecutionRepository(db.DB),
		queue: queue.NewEngine(db.DB, queue.Timings{}),
	}
	f.runs = runs.New(f.queue, queue.NewStats(db.DB), repos.NewRunRepository(db.DB), f.execs, f.gw)
	keys := repos.NewAPIKeyRepository(db.DB)

	r := gin.New()
	panel := r.Group("/api/v1", func(c *gin.Context) { c.Set("user_id", c.GetHeader(testUserHeader)) })
	SetupAPIKeyRoutes(panel, NewAPIKeysHandler(keys, f.users))

	wh := worker.NewWorkerHandler(&config.Config{}, repos.NewWorkerRepository(db.DB), nil, nil, &luteGrpc.Server{})
	SetupPublicRoutes(r.Group(publicBase), keys, Handlers{
		Runs: NewRunsHandler(f.runs),
		Jobs: NewJobsHandler(f.defs, f.runs),
		Meta: NewMetaHandler(keys, f.users),
	}, wh)
	f.router = r
	return f
}

func (f *fixture) user(email string) *models.User {
	f.t.Helper()
	u := &models.User{Email: email, DisplayName: email}
	if err := f.users.Create(context.Background(), u); err != nil {
		f.t.Fatalf("create user: %v", err)
	}
	return u
}

// createdKey is the panel's answer to creating a key.
type createdKey struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Scope  string `json:"scope"`
	Prefix string `json:"prefix"`
	Token  string `json:"token"`
}

// key creates a key as u through the panel API; scope is "account" or "service".
func (f *fixture) key(u *models.User, name, scope string) createdKey {
	f.t.Helper()
	rec := f.panel(u, http.MethodPost, "/api/v1/api-keys", map[string]string{"name": name, "scope": scope})
	if rec.Code != http.StatusCreated {
		f.t.Fatalf("create %s key: %d %s", scope, rec.Code, rec.Body)
	}
	return decode[createdKey](f.t, rec)
}

func (f *fixture) job(def *models.JobDefinition) *models.JobDefinition {
	f.t.Helper()
	if def.Queue == "" {
		def.Queue = "build"
	}
	if def.Runtime == "" {
		def.Runtime = "bash:5"
	}
	if def.Name == "" {
		def.Name = def.Slug
	}
	if err := f.defs.Create(context.Background(), def); err != nil {
		f.t.Fatalf("create job definition: %v", err)
	}
	return def
}

// panel calls a panel route as u.
func (f *fixture) panel(u *models.User, method, path string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	return f.do(method, path, body, http.Header{testUserHeader: {u.ID.Hex()}})
}

// public calls a public route with token; an empty token sends no Authorization header.
func (f *fixture) public(token, method, path string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	h := http.Header{}
	if token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
	return f.do(method, publicBase+path, body, h)
}

func (f *fixture) do(method, path string, body any, header http.Header) *httptest.ResponseRecorder {
	f.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if raw, ok := body.(string); ok {
			buf.WriteString(raw)
		} else if err := json.NewEncoder(&buf).Encode(body); err != nil {
			f.t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range header {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return out
}

// apiError is the shared error shape, httpx.Body.
type apiError struct {
	Error struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields"`
	} `json:"error"`
}

// wantError fails unless rec is an error with this status and code.
func wantError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) apiError {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d: %s", rec.Code, status, rec.Body)
	}
	e := decode[apiError](t, rec)
	if e.Error.Code != code {
		t.Fatalf("error code = %q, want %q: %s", e.Error.Code, code, rec.Body)
	}
	return e
}
