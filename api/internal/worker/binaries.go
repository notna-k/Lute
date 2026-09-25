package worker

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/httpx"
)

type WorkerBinaryInfo struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
}

func (h *WorkerHandler) ListBinaries(c *gin.Context) {
	h.binaryMu.RLock()
	defer h.binaryMu.RUnlock()

	binaries := make([]*WorkerBinaryInfo, 0, len(h.binaryCache))
	for _, binaryInfo := range h.binaryCache {
		binaries = append(binaries, binaryInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"binaries": binaries,
		"version":  readVersionFile(h.binaryDir),
	})
}

func (h *WorkerHandler) DownloadBinary(c *gin.Context) {
	h.serveBinary(c, c.Param("os"), c.Param("arch"))
}

func (h *WorkerHandler) DownloadAutoDetect(c *gin.Context) {
	h.serveBinary(c, c.DefaultQuery("os", "linux"), c.DefaultQuery("arch", "amd64"))
}

func (h *WorkerHandler) serveBinary(c *gin.Context, osName, arch string) {
	cacheKey := osName + "/" + arch

	h.binaryMu.RLock()
	binaryInfo, ok := h.binaryCache[cacheKey]
	var available []string
	if !ok {
		for key := range h.binaryCache {
			available = append(available, key)
		}
	}
	h.binaryMu.RUnlock()

	if !ok {
		sort.Strings(available)
		httpx.Error(c, http.StatusNotFound, fmt.Sprintf("no worker binary for %s/%s (available: %s)", osName, arch, strings.Join(available, ", ")))
		return
	}

	fullPath := filepath.Join(h.binaryDir, binaryInfo.Filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", binaryInfo.Filename))
	c.Header("X-Worker-Version", binaryInfo.Version)
	c.Header("X-Worker-SHA256", binaryInfo.SHA256)
	c.File(fullPath)
}

func (h *WorkerHandler) GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": readVersionFile(h.binaryDir)})
}

func (h *WorkerHandler) RefreshBinaries(c *gin.Context) {
	h.refreshBinaryCache()

	h.binaryMu.RLock()
	count := len(h.binaryCache)
	h.binaryMu.RUnlock()

	c.JSON(http.StatusOK, gin.H{"message": "Binary cache refreshed", "count": count})
}

func (h *WorkerHandler) refreshBinaryCache() {
	h.binaryMu.Lock()
	defer h.binaryMu.Unlock()

	entries, err := os.ReadDir(h.binaryDir)
	if err != nil {
		slog.Warn("cannot read worker binary dir", "dir", h.binaryDir, "err", err)
		return
	}

	newCache := make(map[string]*WorkerBinaryInfo)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		if !strings.HasPrefix(filename, "lute-worker-") {
			continue
		}

		osName, arch := parseWorkerFilename(filename)
		if osName == "" || arch == "" {
			continue
		}

		fullPath := filepath.Join(h.binaryDir, filename)
		fileInfo, err := entry.Info()
		if err != nil {
			continue
		}

		checksum, err := sha256File(fullPath)
		if err != nil {
			slog.Warn("cannot checksum worker binary", "file", filename, "err", err)
			continue
		}

		cacheKey := osName + "/" + arch
		newCache[cacheKey] = &WorkerBinaryInfo{
			OS:       osName,
			Arch:     arch,
			Version:  readVersionFile(h.binaryDir),
			Filename: filename,
			SHA256:   checksum,
			Size:     fileInfo.Size(),
		}
		slog.Info("indexed worker binary", "file", filename, "os", osName, "arch", arch, "bytes", fileInfo.Size())
	}

	h.binaryCache = newCache
}

func parseWorkerFilename(filename string) (osName, arch string) {
	filename = strings.TrimSuffix(filename, ".exe")
	parts := strings.Split(filename, "-")
	if len(parts) < 4 {
		return "", ""
	}
	return parts[2], parts[3]
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func readVersionFile(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}
