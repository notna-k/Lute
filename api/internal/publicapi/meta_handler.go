package publicapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/httpx"
	"github.com/lute/api/internal/version"
)

// APILevel goes up when the public API changes in a way clients must know about.
const APILevel = 1

// MetaHandler answers what a client asks before real work: which server this is, and
// who its key acts as.
type MetaHandler struct {
	keys  *repos.APIKeyRepository
	users *repos.UserRepository
}

func NewMetaHandler(keys *repos.APIKeyRepository, users *repos.UserRepository) *MetaHandler {
	return &MetaHandler{keys: keys, users: users}
}

type VersionResponse struct {
	Version  string `json:"version"`
	APILevel int    `json:"api_level"`
}

type WhoAmIResponse struct {
	Key KeyInfo `json:"key"`
	// User is who an account key acts as; a service key acts as itself and has none.
	User *UserInfo `json:"user,omitempty"`
}

type KeyInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
	Scope  string `json:"scope"`
}

type UserInfo struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name,omitempty"`
}

// Version needs no key, so a client can check the server before it has one.
func (h *MetaHandler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, VersionResponse{Version: version.Core, APILevel: APILevel})
}

func (h *MetaHandler) WhoAmI(c *gin.Context) {
	caller, ok := httpx.Key(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	k, err := h.keys.GetByID(ctx, caller.KeyID)
	if err != nil {
		httpx.Internal(c, err)
		return
	}

	resp := WhoAmIResponse{Key: KeyInfo{ID: k.ID.Hex(), Name: k.Name, Prefix: k.Prefix, Scope: k.Scope}}
	if !caller.Service {
		u, err := h.users.GetByID(ctx, caller.UserID)
		if err != nil {
			httpx.Internal(c, err)
			return
		}
		resp.User = &UserInfo{ID: u.ID.Hex(), Email: u.Email, DisplayName: u.DisplayName}
	}
	c.JSON(http.StatusOK, resp)
}
