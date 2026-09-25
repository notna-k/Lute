package runs

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/httpx"
	"github.com/lute/api/internal/queue"
)

// WriteError answers a Service error; notFound says what was missing.
func WriteError(c *gin.Context, err error, notFound string) {
	switch {
	case errors.Is(err, repos.ErrNotFound):
		httpx.Error(c, http.StatusNotFound, notFound)
	case errors.Is(err, ErrNoLogs):
		httpx.Error(c, http.StatusNotFound, err.Error())
	case errors.Is(err, queue.ErrNotCancellable):
		httpx.Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, ErrWorkerOffline):
		httpx.Error(c, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, ErrLogRead):
		httpx.Error(c, http.StatusBadGateway, err.Error())
	default:
		httpx.Internal(c, err)
	}
}

// ReadLogQuery parses the log paging parameters, or answers 400.
func ReadLogQuery(c *gin.Context) (LogQuery, bool) {
	q, err := ParseLogQuery(c.Query("direction"), c.Query("limit"), c.Query("cursor"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return q, false
	}
	return q, true
}
