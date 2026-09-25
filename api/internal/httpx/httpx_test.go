package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/repos"
)

func init() { gin.SetMode(gin.TestMode) }

// serve runs h behind a route and returns the response.
func serve(t *testing.T, h gin.HandlerFunc, pre ...gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.GET("/", append(pre, h)...)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	return w
}

// decode reads the body strictly, so a key outside the contract fails the test.
func decode(t *testing.T, w *httptest.ResponseRecorder) Detail {
	t.Helper()
	dec := json.NewDecoder(w.Body)
	dec.DisallowUnknownFields()
	var b Body
	if err := dec.Decode(&b); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return b.Error
}

func TestErrorPicksTheCodeFromTheStatus(t *testing.T) {
	cases := []struct {
		status int
		code   string
	}{
		{http.StatusBadRequest, CodeBadRequest},
		{http.StatusUnauthorized, CodeUnauthorized},
		{http.StatusForbidden, CodeForbidden},
		{http.StatusNotFound, CodeNotFound},
		{http.StatusConflict, CodeConflict},
		{http.StatusBadGateway, CodeUnavailable},
		{http.StatusServiceUnavailable, CodeUnavailable},
		{http.StatusInternalServerError, CodeInternal},
		{http.StatusUnprocessableEntity, CodeBadRequest},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			w := serve(t, func(c *gin.Context) { Error(c, tc.status, "nope") })
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			got := decode(t, w)
			if got.Code != tc.code || got.Message != "nope" || got.Fields != nil {
				t.Fatalf("body = %+v, want code %q, message nope, no fields", got, tc.code)
			}
		})
	}
}

func TestErrorOmitsFieldsFromTheWire(t *testing.T) {
	w := serve(t, func(c *gin.Context) { Error(c, http.StatusNotFound, "gone") })
	if want := `{"error":{"code":"not_found","message":"gone"}}`; w.Body.String() != want {
		t.Fatalf("body = %s, want %s", w.Body.String(), want)
	}
}

func TestInvalidCarriesFields(t *testing.T) {
	w := serve(t, func(c *gin.Context) {
		Invalid(c, "retries must be a number", map[string]string{"retries": "must be a number"})
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	got := decode(t, w)
	if got.Code != CodeValidation || got.Fields["retries"] != "must be a number" {
		t.Fatalf("body = %+v", got)
	}
}

func TestErrorStopsTheChain(t *testing.T) {
	reached := false
	r := gin.New()
	r.GET("/", func(c *gin.Context) { Error(c, http.StatusUnauthorized, "no") }, func(*gin.Context) { reached = true })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if reached {
		t.Fatal("a handler after the error still ran")
	}
}

func TestNotFoundOrInternal(t *testing.T) {
	w := serve(t, func(c *gin.Context) {
		NotFoundOrInternal(c, fmt.Errorf("load: %w", repos.ErrNotFound), "job not found")
	})
	if got := decode(t, w); w.Code != http.StatusNotFound || got.Message != "job not found" {
		t.Fatalf("wrapped ErrNotFound: %d %+v, want 404 job not found", w.Code, got)
	}

	var logged []string
	w = serve(t, func(c *gin.Context) {
		NotFoundOrInternal(c, errors.New("connection reset"), "job not found")
		logged = c.Errors.Errors()
	})
	if got := decode(t, w); w.Code != http.StatusInternalServerError || got.Code != CodeInternal {
		t.Fatalf("other error: %d %+v, want 500 internal", w.Code, got)
	}
	if len(logged) != 1 {
		t.Fatalf("the internal error was not recorded for the request log: %v", logged)
	}
}

func TestUserID(t *testing.T) {
	valid := id.New()
	cases := []struct {
		name string
		set  any
		ok   bool
	}{
		{"missing", nil, false},
		{"not hex", "user-1", false},
		{"not a string", 42, false},
		{"valid", valid.Hex(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got id.ID
			var ok bool
			w := serve(t, func(c *gin.Context) {
				if got, ok = UserID(c); ok {
					c.Status(http.StatusNoContent)
				}
			}, func(c *gin.Context) {
				if tc.set != nil {
					c.Set("user_id", tc.set)
				}
			})
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if tc.ok {
				if got != valid {
					t.Fatalf("id = %q, want %q", got, valid)
				}
				return
			}
			if w.Code != http.StatusUnauthorized || decode(t, w).Code != CodeUnauthorized {
				t.Fatalf("status = %d, want 401 unauthorized", w.Code)
			}
		})
	}
}
