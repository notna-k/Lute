// Package httpx holds what every HTTP handler shares: the error shape and the caller's identity.
package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/repos"
)

// Error codes. Clients branch on these, never on the message.
const (
	CodeBadRequest   = "bad_request"
	CodeValidation   = "validation_failed"
	CodeUnauthorized = "unauthorized"
	CodeForbidden    = "forbidden"
	CodeNotFound     = "not_found"
	CodeConflict     = "conflict"
	CodeUnavailable  = "unavailable"
	CodeInternal     = "internal"
)

// Body is every error response: {"error": {"code", "message", "fields"}}.
type Body struct {
	Error Detail `json:"error"`
}

type Detail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// Fields maps an input to what is wrong with it; only validation errors carry it.
	Fields map[string]string `json:"fields,omitempty"`
}

// Error aborts with message and the code that goes with status.
func Error(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, Body{Detail{Code: codeFor(status), Message: message}})
}

// Invalid aborts with 400 validation_failed and per-input messages.
func Invalid(c *gin.Context, message string, fields map[string]string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, Body{Detail{Code: CodeValidation, Message: message, Fields: fields}})
}

// Internal aborts with 500 and records err for the request log; clients never see its text.
func Internal(c *gin.Context, err error) {
	_ = c.Error(err)
	Error(c, http.StatusInternalServerError, "internal server error")
}

// NotFoundOrInternal answers 404 with notFound for repos.ErrNotFound, else 500.
func NotFoundOrInternal(c *gin.Context, err error, notFound string) {
	if errors.Is(err, repos.ErrNotFound) {
		Error(c, http.StatusNotFound, notFound)
		return
	}
	Internal(c, err)
}

// UserID returns the caller the auth middleware set, or aborts with 401.
func UserID(c *gin.Context) (id.ID, bool) {
	uid, err := id.FromHex(c.GetString("user_id"))
	if err != nil {
		Error(c, http.StatusUnauthorized, "authentication required")
		return "", false
	}
	return uid, true
}

func codeFor(status int) string {
	switch status {
	case http.StatusBadRequest:
		return CodeBadRequest
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusConflict:
		return CodeConflict
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return CodeUnavailable
	}
	if status >= 500 {
		return CodeInternal
	}
	return CodeBadRequest
}
