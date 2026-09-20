package settings

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
)

// Handler serves the panel-managed operator settings.
type Handler struct {
	settings *repos.SettingRepository
}

func NewHandler(settings *repos.SettingRepository) *Handler {
	return &Handler{settings: settings}
}

// settingsDTO is the panel's view of every knob. Keys are explicit fields
// rather than a bare map so the UI has a typed contract.
type settingsDTO struct {
	AllowAdhocBuilds bool `json:"allowAdhocBuilds"`
	PruneDefinitions bool `json:"pruneDefinitions"`
}

// Get returns the current settings, with defaults for anything never written.
func (h *Handler) Get(c *gin.Context) {
	all, err := h.settings.All(c.Request.Context())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	allow, _ := strconv.ParseBool(all[models.AllowAdhocBuilds])
	prune, _ := strconv.ParseBool(all[models.PruneDefinitions])
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, settingsDTO{AllowAdhocBuilds: allow, PruneDefinitions: prune})
}

// updateRequest uses pointers so an omitted field means "leave unchanged"
// rather than "set to false".
type updateRequest struct {
	AllowAdhocBuilds *bool `json:"allowAdhocBuilds"`
	PruneDefinitions *bool `json:"pruneDefinitions"`
}

// Update writes the provided settings and returns the full resulting state.
func (h *Handler) Update(c *gin.Context) {
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	for key, v := range map[string]*bool{
		models.AllowAdhocBuilds: req.AllowAdhocBuilds,
		models.PruneDefinitions: req.PruneDefinitions,
	} {
		if v == nil {
			continue
		}
		if err := h.settings.Set(c.Request.Context(), key, strconv.FormatBool(*v)); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	h.Get(c)
}
