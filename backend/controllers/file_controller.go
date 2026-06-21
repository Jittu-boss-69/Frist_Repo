package controllers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"devsync/backend/config"
	"devsync/backend/models"
	"devsync/backend/repository"
	"devsync/backend/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type FileController struct {
	fileRepo     *repository.SharedFileRepository
	activityRepo *repository.ActivityLogRepository
}

func NewFileController(fRepo *repository.SharedFileRepository, actRepo *repository.ActivityLogRepository) *FileController {
	return &FileController{
		fileRepo:     fRepo,
		activityRepo: actRepo,
	}
}

// UploadSingle handles single-file uploads (for CLI/curl or small files)
func (ctrl *FileController) UploadSingle(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file parameter found"})
		return
	}

	senderDevice := c.DefaultPostForm("sender_device", c.ClientIP())
	relativePath := c.DefaultPostForm("relative_path", "")
	fileUUID := uuid.New()
	safeFileName := fileHeader.Filename
	savedName := fmt.Sprintf("%s_%s", fileUUID.String(), safeFileName)
	destPath := filepath.Join(config.AppConfig.UploadDir, savedName)

	if err := c.SaveUploadedFile(fileHeader, destPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	sharedFile := &models.SharedFile{
		ID:            fileUUID,
		FileName:      safeFileName,
		FilePath:      destPath,
		FileSize:      fileHeader.Size,
		FileType:      filepath.Ext(safeFileName),
		SenderDevice:  senderDevice,
		DownloadCount: 0,
		RelativePath:  relativePath,
		UploadedAt:    time.Now(),
	}

	if err := ctrl.fileRepo.Create(sharedFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata"})
		return
	}

	// Log activity
	logEntry := &models.ActivityLog{
		ID:           uuid.New(),
		ActivityType: "file_upload",
		Description:  fmt.Sprintf("Device %s uploaded file: %s (%s)", senderDevice, safeFileName, formatBytes(fileHeader.Size)),
		CreatedAt:    time.Now(),
	}
	_ = ctrl.activityRepo.Create(logEntry)

	// Broadcast WS updates
	websocket.GlobalHub.Broadcast("new_file", sharedFile)
	websocket.GlobalHub.Broadcast("activity", logEntry)

	c.JSON(http.StatusOK, gin.H{
		"message": "File uploaded successfully",
		"file":    sharedFile,
	})
}

// UploadChunk handles uploading individual chunks of a large file
func (ctrl *FileController) UploadChunk(c *gin.Context) {
	chunkHeader, err := c.FormFile("chunk")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chunk file is required"})
		return
	}

	uploadUUIDStr := c.PostForm("upload_uuid")
	chunkIndexStr := c.PostForm("chunk_index")
	totalChunksStr := c.PostForm("total_chunks")

	if uploadUUIDStr == "" || chunkIndexStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "upload_uuid and chunk_index are required"})
		return
	}

	_, err = uuid.Parse(uploadUUIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid upload_uuid"})
		return
	}

	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chunk_index"})
		return
	}

	// Write chunk to a temp path
	chunkName := fmt.Sprintf("%s_%d", uploadUUIDStr, chunkIndex)
	chunkPath := filepath.Join(config.AppConfig.UploadDir, "temp_chunks", chunkName)

	if err := c.SaveUploadedFile(chunkHeader, chunkPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save chunk"})
		return
	}

	// Broadcast upload progress back to all UI clients for the active uploads dashboard widget
	websocket.GlobalHub.Broadcast("upload_progress", gin.H{
		"upload_uuid":  uploadUUIDStr,
		"chunk_index":  chunkIndex,
		"total_chunks": totalChunksStr,
		"chunk_size":   chunkHeader.Size,
	})

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Chunk %d uploaded successfully", chunkIndex)})
}

type MergeChunksInput struct {
	UploadUUID   string `json:"upload_uuid" binding:"required"`
	FileName     string `json:"file_name" binding:"required"`
	TotalChunks  int    `json:"total_chunks" binding:"required"`
	FileSize     int64  `json:"file_size" binding:"required"`
	SenderDevice string `json:"sender_device"`
	RelativePath string `json:"relative_path"`
}

// MergeChunks merges all uploaded chunks into a single file
func (ctrl *FileController) MergeChunks(c *gin.Context) {
	var input MergeChunksInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uploadUUID, err := uuid.Parse(input.UploadUUID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid upload_uuid"})
		return
	}

	savedName := fmt.Sprintf("%s_%s", uploadUUID.String(), input.FileName)
	destPath := filepath.Join(config.AppConfig.UploadDir, savedName)

	destFile, err := os.Create(destPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create destination file"})
		return
	}
	defer destFile.Close()

	// Append each chunk
	for i := 0; i < input.TotalChunks; i++ {
		chunkName := fmt.Sprintf("%s_%d", input.UploadUUID, i)
		chunkPath := filepath.Join(config.AppConfig.UploadDir, "temp_chunks", chunkName)

		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to open chunk %d", i)})
			return
		}

		_, err = io.Copy(destFile, chunkFile)
		chunkFile.Close()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to merge chunk %d", i)})
			return
		}

		// Delete chunk file immediately to save disk space
		_ = os.Remove(chunkPath)
	}

	sender := input.SenderDevice
	if sender == "" {
		sender = c.ClientIP()
	}

	sharedFile := &models.SharedFile{
		ID:            uploadUUID,
		FileName:      input.FileName,
		FilePath:      destPath,
		FileSize:      input.FileSize,
		FileType:      filepath.Ext(input.FileName),
		SenderDevice:  sender,
		DownloadCount: 0,
		RelativePath:  input.RelativePath,
		UploadedAt:    time.Now(),
	}

	if err := ctrl.fileRepo.Create(sharedFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata"})
		return
	}

	// Log activity
	logEntry := &models.ActivityLog{
		ID:           uuid.New(),
		ActivityType: "file_upload",
		Description:  fmt.Sprintf("Device %s uploaded file: %s (%s)", sender, input.FileName, formatBytes(input.FileSize)),
		CreatedAt:    time.Now(),
	}
	_ = ctrl.activityRepo.Create(logEntry)

	websocket.GlobalHub.Broadcast("new_file", sharedFile)
	websocket.GlobalHub.Broadcast("activity", logEntry)

	c.JSON(http.StatusOK, sharedFile)
}

func (ctrl *FileController) List(c *gin.Context) {
	searchQuery := c.Query("q")
	fileType := c.Query("type")

	list, err := ctrl.fileRepo.GetAll(searchQuery, fileType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list files"})
		return
	}

	c.JSON(http.StatusOK, list)
}

func (ctrl *FileController) Download(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid UUID format"})
		return
	}

	sharedFile, err := ctrl.fileRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if sharedFile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Check if file exists on disk
	if _, err := os.Stat(sharedFile.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Physical file not found on server disk"})
		return
	}

	// Update download count
	_ = ctrl.fileRepo.IncrementDownloadCount(id)

	// Log download activity
	ip := c.ClientIP()
	logEntry := &models.ActivityLog{
		ID:           uuid.New(),
		ActivityType: "file_download",
		Description:  fmt.Sprintf("IP %s downloaded file: %s", ip, sharedFile.FileName),
		CreatedAt:    time.Now(),
	}
	_ = ctrl.activityRepo.Create(logEntry)
	
	// Notify WebSocket clients
	websocket.GlobalHub.Broadcast("file_download", gin.H{"id": idStr, "download_count": sharedFile.DownloadCount + 1})
	websocket.GlobalHub.Broadcast("activity", logEntry)

	// Set attachment header so browser prompts download instead of inline displaying
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+sharedFile.FileName)
	c.Header("Content-Type", "application/octet-stream")

	c.File(sharedFile.FilePath)
}

func (ctrl *FileController) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid UUID"})
		return
	}

	sharedFile, err := ctrl.fileRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if sharedFile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Delete from disk
	_ = os.Remove(sharedFile.FilePath)

	// Delete from DB
	if err := ctrl.fileRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete database entry"})
		return
	}

	// Log activity
	logEntry := &models.ActivityLog{
		ID:           uuid.New(),
		ActivityType: "file_delete",
		Description:  "Deleted file: " + sharedFile.FileName,
		CreatedAt:    time.Now(),
	}
	_ = ctrl.activityRepo.Create(logEntry)

	websocket.GlobalHub.Broadcast("file_delete", idStr)
	websocket.GlobalHub.Broadcast("activity", logEntry)

	c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
}

// Helpers
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
