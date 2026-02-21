package handlers

import (
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/lute/api/models"
	"github.com/lute/api/repository"
	"github.com/lute/api/services"
)

// DashboardHandler handles dashboard stats and uptime API.
type DashboardHandler struct {
	machineService *services.MachineService
	snapshotRepo   *repository.MachineSnapshotRepository
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(machineService *services.MachineService, snapshotRepo *repository.MachineSnapshotRepository) *DashboardHandler {
	return &DashboardHandler{
		machineService: machineService,
		snapshotRepo:   snapshotRepo,
	}
}

// GetStats handles GET /api/v1/dashboard/stats (authenticated).
// Returns { total, alive, dead, public } for the current user.
func (h *DashboardHandler) GetStats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userIDObj, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	ctx := c.Request.Context()
	machines, err := h.machineService.GetByUserID(ctx, userIDObj)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var alive, dead int
	for _, m := range machines {
		switch m.Status {
		case "alive":
			alive++
		case "dead":
			dead++
		}
	}
	total := len(machines)

	publicMachines, err := h.machineService.GetPublic(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	publicCount := len(publicMachines)

	c.JSON(http.StatusOK, gin.H{
		"total":  total,
		"alive":  alive,
		"dead":   dead,
		"public": publicCount,
	})
}

// UptimePoint is one point in the uptime time series (aggregated or per-machine; same metrics schema as Machine.Metrics).
type UptimePoint struct {
	At        string                 `json:"at"`
	Status    string                 `json:"status,omitempty"`
	UptimePct float64                `json:"uptime_pct,omitempty"`
	Metrics   map[string]interface{} `json:"metrics"` // cpu_load, mem_usage_mb, disk_used_gb, disk_total_gb
}

// GetUptime handles GET /api/v1/dashboard/uptime?period=7d (authenticated).
// Returns { points: [ { at, uptime_pct, metrics } ] } aggregated across the user's machines.
func (h *DashboardHandler) GetUptime(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userIDObj, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	period := c.DefaultQuery("period", "7d")
	var since time.Time
	switch period {
	case "24h":
		since = time.Now().Add(-24 * time.Hour)
	case "7d":
		since = time.Now().Add(-7 * 24 * time.Hour)
	default:
		since = time.Now().Add(-7 * 24 * time.Hour)
	}

	ctx := c.Request.Context()
	machines, err := h.machineService.GetByUserID(ctx, userIDObj)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(machines) == 0 {
		c.JSON(http.StatusOK, gin.H{"points": []UptimePoint{}})
		return
	}
	machineIDs := make([]primitive.ObjectID, 0, len(machines))
	for _, m := range machines {
		machineIDs = append(machineIDs, m.ID)
	}
	snapshots, err := h.snapshotRepo.GetByMachineIDs(ctx, machineIDs, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Aggregate by at: for each timestamp, compute uptime_pct and average metrics
	byAt := make(map[time.Time][]*models.MachineSnapshot)
	for _, s := range snapshots {
		t := s.At.Truncate(time.Second)
		byAt[t] = append(byAt[t], s)
	}
	ats := make([]time.Time, 0, len(byAt))
	for t := range byAt {
		ats = append(ats, t)
	}
	sort.Slice(ats, func(i, j int) bool { return ats[i].Before(ats[j]) })
	points := make([]UptimePoint, 0, len(ats))
	for _, t := range ats {
		list := byAt[t]
		alive := 0
		var sumCpu, sumMem, sumDiskUsed, sumDiskTotal float64
		for _, s := range list {
			if s.Status == "alive" {
				alive++
			}
			sumCpu += floatFrom(s.Metrics, "cpu_load")
			sumMem += floatFrom(s.Metrics, "mem_usage_mb")
			sumDiskUsed += floatFrom(s.Metrics, "disk_used_gb")
			sumDiskTotal += floatFrom(s.Metrics, "disk_total_gb")
		}
		n := float64(len(list))
		pct := 0.0
		if n > 0 {
			pct = float64(alive) / n * 100
		}
		points = append(points, UptimePoint{
			At:        t.Format(time.RFC3339),
			UptimePct: pct,
			Metrics: map[string]interface{}{
				"cpu_load":      roundMetric(sumCpu / n),
				"mem_usage_mb":  roundMetric(sumMem / n),
				"disk_used_gb":  roundMetric(sumDiskUsed / n),
				"disk_total_gb": roundMetric(sumDiskTotal / n),
			},
		})
	}
	c.JSON(http.StatusOK, gin.H{"points": points})
}

func floatFrom(m map[string]interface{}, key string) float64 {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

func roundMetric(v float64) float64 {
	return float64(int(v*1000+0.5)) / 1000
}
