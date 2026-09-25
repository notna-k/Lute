package settings

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
)

type Handler struct {
	settings *repos.SettingRepository
}

func NewHandler(settings *repos.SettingRepository) *Handler {
	return &Handler{settings: settings}
}

type settingsDTO struct {
	AllowAdhocBuilds bool `json:"allowAdhocBuilds"`
	PruneDefinitions bool `json:"pruneDefinitions"`
}

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

// updateRequest uses pointers so an omitted field is left unchanged, not set to false.
type updateRequest struct {
	AllowAdhocBuilds *bool `json:"allowAdhocBuilds"`
	PruneDefinitions *bool `json:"pruneDefinitions"`
}

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
