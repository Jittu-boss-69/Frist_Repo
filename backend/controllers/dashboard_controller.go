package controllers

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"devsync/backend/config"
	"devsync/backend/repository"
	"devsync/backend/utils"
	"github.com/gin-gonic/gin"
)

type DashboardController struct {
	fileRepo     *repository.SharedFileRepository
	snippetRepo  *repository.SnippetRepository
	deviceRepo   *repository.DeviceRepository
	activityRepo *repository.ActivityLogRepository
}

func NewDashboardController(
	fRepo *repository.SharedFileRepository,
	sRepo *repository.SnippetRepository,
	dRepo *repository.DeviceRepository,
	aRepo *repository.ActivityLogRepository,
) *DashboardController {
	return &DashboardController{
		fileRepo:     fRepo,
		snippetRepo:  sRepo,
		deviceRepo:   dRepo,
		activityRepo: aRepo,
	}
}

// GetMetrics returns general stats, system memory usage, host IP, and recent activities
func (ctrl *DashboardController) GetMetrics(c *gin.Context) {
	// 1. Files & Snippets stats
	files, err := ctrl.fileRepo.GetAll("", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load files data"})
		return
	}

	snippets, err := ctrl.snippetRepo.GetAll("", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load snippets data"})
		return
	}

	devices, err := ctrl.deviceRepo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load devices data"})
		return
	}

	activities, err := ctrl.activityRepo.GetRecent(15)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load activity logs"})
		return
	}

	// 2. Count metrics
	totalFiles := len(files)
	totalSnippets := len(snippets)
	onlineDevicesCount := 0
	for _, d := range devices {
		if d.Status == "online" {
			onlineDevicesCount++
		}
	}

	// 3. Storage Usage
	var storageUsage int64 = 0
	err = filepath.Walk(config.AppConfig.UploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			storageUsage += info.Size()
		}
		return nil
	})
	if err != nil {
		storageUsage = 0
	}

	// 4. Memory Usage (Server CPU/RAM stats)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memoryAllocated := m.Alloc // bytes

	// 5. Host Local Network IP
	localIP := utils.GetLocalIP()

	c.JSON(http.StatusOK, gin.H{
		"total_files":          totalFiles,
		"total_snippets":       totalSnippets,
		"online_devices_count": onlineDevicesCount,
		"storage_usage_bytes":  storageUsage,
		"memory_allocated_mb":  float64(memoryAllocated) / 1024.0 / 1024.0,
		"local_ip":             localIP,
		"port":                 config.AppConfig.Port,
		"connection_url":       "http://" + localIP + ":" + config.AppConfig.Port,
		"activities":           activities,
		"devices":              devices,
	})
}

// HealthCheck simple handler for docker or ping probes
func (ctrl *DashboardController) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}
