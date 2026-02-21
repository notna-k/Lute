package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/lute/api/config"
	"github.com/lute/api/models"
	"github.com/lute/api/repository"
	"github.com/lute/api/services"
)

// DashboardHandler handles dashboard stats and uptime API.
type DashboardHandler struct {
	cfg            *config.Config
	machineService *services.MachineService
	snapshotRepo   *repository.MachineSnapshotRepository
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(cfg *config.Config, machineService *services.MachineService, snapshotRepo *repository.MachineSnapshotRepository) *DashboardHandler {
	return &DashboardHandler{
		cfg:            cfg,
		machineService: machineService,
		snapshotRepo:   snapshotRepo,
	}
}

// GetConfig returns dashboard/metrics client config (e.g. poll interval to match snapshot job).
func (h *DashboardHandler) GetConfig(c *gin.Context) {
	sec := int(h.cfg.Metrics.SnapshotInterval.Seconds())
	if sec < 1 {
		sec = 1
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"metrics_poll_interval_seconds": sec})
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

// ChartPoint is one bucket-aligned point for charts (t=unix ms, null metrics = gap / machine was down).
type ChartPoint struct {
	T           int64    `json:"t"`
	CpuLoad     *float64 `json:"cpu_load"`
	MemUsageMb  *float64 `json:"mem_usage_mb"`
	DiskUsedGb  *float64 `json:"disk_used_gb"`
	DiskTotalGb *float64 `json:"disk_total_gb"`
}

// ChartResponse is the dashboard uptime API response (chart-ready, backend-bucketed).
type ChartResponse struct {
	Points        []ChartPoint `json:"points"`
	PeriodStartMs int64        `json:"period_start_ms"`
	PeriodEndMs   int64        `json:"period_end_ms"`
	DiskYDomain   [2]float64  `json:"disk_y_domain"`
}

// targetChartPoints is the desired number of data points for any period.
const targetChartPoints = 80

// bucketDuration returns the chart bucket size for the given period.
// It targets ~targetChartPoints buckets regardless of snapshot interval.
// The bucket is snapped up to snapshotInterval so buckets are never smaller than
// the collection resolution (which would produce guaranteed-empty buckets).
func bucketDuration(period string, snapshotInterval time.Duration) time.Duration {
	if snapshotInterval <= 0 {
		snapshotInterval = time.Minute
	}
	var periodDur time.Duration
	switch period {
	case "10m":
		periodDur = 10 * time.Minute
	case "1h":
		periodDur = time.Hour
	case "24h":
		periodDur = 24 * time.Hour
	case "7d":
		periodDur = 7 * 24 * time.Hour
	default:
		periodDur = 7 * 24 * time.Hour
	}
	bucket := periodDur / targetChartPoints
	if bucket < snapshotInterval {
		bucket = snapshotInterval
	}
	return bucket
}

func buildChartPerMachine(snapshots []*models.MachineSnapshot, periodStart, periodEnd time.Time, bucketDur time.Duration) (points []ChartPoint, diskMax float64) {
	bucketMs := bucketDur.Milliseconds()
	periodStartMs := periodStart.UnixMilli()
	periodEndMs := periodEnd.UnixMilli()

	// Group by bucket, keep latest snapshot per bucket (handles ticker jitter).
	type bucketVal struct {
		at                      time.Time
		cpu, mem, diskUsed, diskTotal float64
	}
	byBucket := make(map[int64]*bucketVal)
	for _, s := range snapshots {
		tMs := s.At.UnixMilli()
		b := (tMs / bucketMs) * bucketMs
		existing, ok := byBucket[b]
		if !ok || s.At.After(existing.at) {
			byBucket[b] = &bucketVal{
				at:        s.At,
				cpu:       floatFrom(s.Metrics, "cpu_load"),
				mem:       floatFrom(s.Metrics, "mem_usage_mb"),
				diskUsed:  floatFrom(s.Metrics, "disk_used_gb"),
				diskTotal: floatFrom(s.Metrics, "disk_total_gb"),
			}
		}
	}

	// Emit one point per bucket. Null metrics = machine was down (gap).
	diskMax = 1
	for b := periodStartMs; b <= periodEndMs; b += bucketMs {
		p := ChartPoint{T: b}
		if v, ok := byBucket[b]; ok {
			p.CpuLoad = ptrFloat(roundMetric(v.cpu))
			p.MemUsageMb = ptrFloat(roundMetric(v.mem))
			p.DiskUsedGb = ptrFloat(roundMetric(v.diskUsed))
			p.DiskTotalGb = ptrFloat(roundMetric(v.diskTotal))
			if v.diskTotal > diskMax {
				diskMax = v.diskTotal
			}
		}
		points = append(points, p)
	}
	return points, diskMax
}

func buildChartAggregated(snapshots []*models.MachineSnapshot, periodStart, periodEnd time.Time, bucketDur time.Duration) (points []ChartPoint, diskMax float64) {
	bucketMs := bucketDur.Milliseconds()
	periodStartMs := periodStart.UnixMilli()
	periodEndMs := periodEnd.UnixMilli()

	type agg struct {
		n            int
		sumCpu       float64
		sumMem       float64
		sumDiskUsed  float64
		sumDiskTotal float64
	}
	byBucket := make(map[int64]*agg)
	for _, s := range snapshots {
		tMs := s.At.UnixMilli()
		b := (tMs / bucketMs) * bucketMs
		a, ok := byBucket[b]
		if !ok {
			a = &agg{}
			byBucket[b] = a
		}
		a.n++
		a.sumCpu += floatFrom(s.Metrics, "cpu_load")
		a.sumMem += floatFrom(s.Metrics, "mem_usage_mb")
		a.sumDiskUsed += floatFrom(s.Metrics, "disk_used_gb")
		a.sumDiskTotal += floatFrom(s.Metrics, "disk_total_gb")
	}

	// Emit one point per bucket. Null metrics = all machines were down (gap).
	diskMax = 1
	for b := periodStartMs; b <= periodEndMs; b += bucketMs {
		p := ChartPoint{T: b}
		if a, ok := byBucket[b]; ok && a.n > 0 {
			n := float64(a.n)
			p.CpuLoad = ptrFloat(roundMetric(a.sumCpu / n))
			p.MemUsageMb = ptrFloat(roundMetric(a.sumMem / n))
			p.DiskUsedGb = ptrFloat(roundMetric(a.sumDiskUsed / n))
			total := a.sumDiskTotal / n
			p.DiskTotalGb = ptrFloat(roundMetric(total))
			if total > diskMax {
				diskMax = total
			}
		}
		points = append(points, p)
	}
	return points, diskMax
}

func ptrFloat(f float64) *float64 { return &f }

// GetUptime handles GET /api/v1/dashboard/uptime?period=7d (optional: machine_id=hex) (authenticated).
// If machine_id is set: returns per-machine points (at, status, uptime_pct 0|100, metrics) after validating ownership.
// If machine_id is absent: returns aggregated points across the user's machines.
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
	now := time.Now()
	bucketDur := bucketDuration(period, h.cfg.Metrics.SnapshotInterval)
	bucketMs := bucketDur.Milliseconds()

	var rawStart time.Time
	switch period {
	case "10m":
		rawStart = now.Add(-10 * time.Minute)
	case "1h":
		rawStart = now.Add(-1 * time.Hour)
	case "24h":
		rawStart = now.Add(-24 * time.Hour)
	case "7d":
		rawStart = now.Add(-7 * 24 * time.Hour)
	default:
		rawStart = now.Add(-7 * 24 * time.Hour)
	}
	// Align periodStart to the bucket boundary so the first bucket in the loop
	// always has data when the machine was alive, avoiding a phantom leading gap.
	periodStart := time.UnixMilli((rawStart.UnixMilli() / bucketMs) * bucketMs)
	periodEnd := time.UnixMilli((now.UnixMilli() / bucketMs) * bucketMs)

	ctx := c.Request.Context()
	machineIDHex := c.Query("machine_id")
	if machineIDHex != "" {
		// Per-machine: validate ownership, bucket snapshots, return chart response
		machineID, err := primitive.ObjectIDFromHex(machineIDHex)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid machine ID"})
			return
		}
		machine, err := h.machineService.GetByID(ctx, machineID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "machine not found"})
			return
		}
		if machine.UserID != userIDObj {
			c.JSON(http.StatusNotFound, gin.H{"error": "machine not found"})
			return
		}
		snapshots, err := h.snapshotRepo.GetByMachineID(ctx, machineID, periodStart)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		points, diskMax := buildChartPerMachine(snapshots, periodStart, periodEnd, bucketDur)
		resp := ChartResponse{
			Points:        points,
			PeriodStartMs: periodStart.UnixMilli(),
			PeriodEndMs:   periodEnd.UnixMilli(),
			DiskYDomain:   [2]float64{0, diskMax},
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, resp)
		return
	}

	// Aggregated: all user's machines
	machines, err := h.machineService.GetByUserID(ctx, userIDObj)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(machines) == 0 {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, ChartResponse{
			Points:        nil,
			PeriodStartMs: periodStart.UnixMilli(),
			PeriodEndMs:   periodEnd.UnixMilli(),
			DiskYDomain:   [2]float64{0, 1},
		})
		return
	}
	machineIDs := make([]primitive.ObjectID, 0, len(machines))
	for _, m := range machines {
		machineIDs = append(machineIDs, m.ID)
	}
	snapshots, err := h.snapshotRepo.GetByMachineIDs(ctx, machineIDs, periodStart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	points, diskMax := buildChartAggregated(snapshots, periodStart, periodEnd, bucketDur)
	resp := ChartResponse{
		Points:        points,
		PeriodStartMs: periodStart.UnixMilli(),
		PeriodEndMs:   periodEnd.UnixMilli(),
		DiskYDomain:   [2]float64{0, diskMax},
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, resp)
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
