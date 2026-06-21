package controllers

import (
	"net/http"
	"time"

	"devsync/backend/models"
	"devsync/backend/repository"
	"devsync/backend/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DeviceController struct {
	deviceRepo   *repository.DeviceRepository
	activityRepo *repository.ActivityLogRepository
}

func NewDeviceController(dRepo *repository.DeviceRepository, actRepo *repository.ActivityLogRepository) *DeviceController {
	return &DeviceController{
		deviceRepo:   dRepo,
		activityRepo: actRepo,
	}
}

type RegisterDeviceInput struct {
	DeviceName string `json:"device_name" binding:"required"`
}

// RegisterOrPing registers a device or updates its last seen / online status
func (ctrl *DeviceController) RegisterOrPing(c *gin.Context) {
	var input RegisterDeviceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ipAddress := c.ClientIP()

	// Find if device already exists
	existing, err := ctrl.deviceRepo.GetByIP(ipAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	device := &models.Device{
		DeviceName: input.DeviceName,
		IPAddress:  ipAddress,
		Status:     "online",
		LastSeen:   time.Now(),
	}

	if existing != nil {
		device.ID = existing.ID
	} else {
		device.ID = uuid.New()
	}

	if err := ctrl.deviceRepo.Upsert(device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log device info"})
		return
	}

	// If it's a new device, log activity and broadcast
	if existing == nil {
		logEntry := &models.ActivityLog{
			ID:           uuid.New(),
			ActivityType: "device_connect",
			Description:  "New device connected: " + device.DeviceName + " (" + device.IPAddress + ")",
			CreatedAt:    time.Now(),
		}
		_ = ctrl.activityRepo.Create(logEntry)
		websocket.GlobalHub.Broadcast("activity", logEntry)
	}

	// Broadcast updated device list
	devices, _ := ctrl.deviceRepo.GetAll()
	websocket.GlobalHub.Broadcast("devices_list", devices)

	c.JSON(http.StatusOK, device)
}

func (ctrl *DeviceController) List(c *gin.Context) {
	list, err := ctrl.deviceRepo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list devices"})
		return
	}
	c.JSON(http.StatusOK, list)
}
