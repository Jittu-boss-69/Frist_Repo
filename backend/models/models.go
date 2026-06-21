package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Snippet struct {
	ID         uuid.UUID  `json:"id"`
	Title      string     `json:"title"`
	Content    string     `json:"content"`
	Language   string     `json:"language"`
	Tags       string     `json:"tags"` // comma-separated
	IsFavorite bool       `json:"is_favorite"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type SharedFile struct {
	ID            uuid.UUID `json:"id"`
	FileName      string    `json:"file_name"`
	FilePath      string    `json:"file_path"`
	FileSize      int64     `json:"file_size"`
	FileType      string    `json:"file_type"`
	SenderDevice  string    `json:"sender_device"`
	DownloadCount int       `json:"download_count"`
	UploadedAt    time.Time `json:"uploaded_at"`
}

type Device struct {
	ID         uuid.UUID `json:"id"`
	DeviceName string    `json:"device_name"`
	IPAddress  string    `json:"ip_address"`
	Status     string    `json:"status"` // "online" or "offline"
	LastSeen   time.Time `json:"last_seen"`
}

type ActivityLog struct {
	ID           uuid.UUID  `json:"id"`
	ActivityType string     `json:"activity_type"`
	Description  string     `json:"description"`
	DeviceID     *uuid.UUID `json:"device_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}
