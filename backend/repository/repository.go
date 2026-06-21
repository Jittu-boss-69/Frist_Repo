package repository

import (
	"database/sql"
	"errors"
	"time"

	"devsync/backend/config"
	"devsync/backend/models"
	"github.com/google/uuid"
)

// ==========================================
// USER REPOSITORY
// ==========================================

type UserRepository struct{}

func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

func (r *UserRepository) Create(u *models.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	u.CreatedAt = time.Now()

	query := `INSERT INTO users (id, username, email, password_hash, created_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := config.DB.Exec(query, u.ID, u.Username, u.Email, u.PasswordHash, u.CreatedAt)
	return err
}

func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	u := &models.User{}
	query := `SELECT id, username, email, password_hash, created_at FROM users WHERE username = $1`
	err := config.DB.QueryRow(query, username).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	u := &models.User{}
	query := `SELECT id, username, email, password_hash, created_at FROM users WHERE email = $1`
	err := config.DB.QueryRow(query, email).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

// ==========================================
// SNIPPET REPOSITORY
// ==========================================

type SnippetRepository struct{}

func NewSnippetRepository() *SnippetRepository {
	return &SnippetRepository{}
}

func (r *SnippetRepository) Create(s *models.Snippet) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	s.CreatedAt = time.Now()

	var expiresVal interface{}
	if s.ExpiresAt != nil {
		expiresVal = *s.ExpiresAt
	} else {
		expiresVal = nil
	}

	query := `INSERT INTO snippets (id, title, content, language, tags, is_favorite, expires_at, created_at) 
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := config.DB.Exec(query, s.ID, s.Title, s.Content, s.Language, s.Tags, s.IsFavorite, expiresVal, s.CreatedAt)
	return err
}

func (r *SnippetRepository) GetByID(id uuid.UUID) (*models.Snippet, error) {
	s := &models.Snippet{}
	var expiresVal sql.NullTime

	query := `SELECT id, title, content, language, tags, is_favorite, expires_at, created_at FROM snippets WHERE id = $1`
	err := config.DB.QueryRow(query, id).Scan(&s.ID, &s.Title, &s.Content, &s.Language, &s.Tags, &s.IsFavorite, &expiresVal, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if expiresVal.Valid {
		s.ExpiresAt = &expiresVal.Time
	}

	return s, nil
}

func (r *SnippetRepository) GetAll(searchQuery string, lang string) ([]models.Snippet, error) {
	var list []models.Snippet
	query := `SELECT id, title, content, language, tags, is_favorite, expires_at, created_at 
			  FROM snippets 
			  WHERE (title ILIKE $1 OR content ILIKE $1 OR tags ILIKE $1) `
	args := []interface{}{"%" + searchQuery + "%"}

	if lang != "" {
		query += " AND language = $2"
		args = append(args, lang)
	}
	query += " ORDER BY created_at DESC"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		s := models.Snippet{}
		var expiresVal sql.NullTime
		err := rows.Scan(&s.ID, &s.Title, &s.Content, &s.Language, &s.Tags, &s.IsFavorite, &expiresVal, &s.CreatedAt)
		if err != nil {
			return nil, err
		}
		if expiresVal.Valid {
			s.ExpiresAt = &expiresVal.Time
		}
		list = append(list, s)
	}
	return list, nil
}

func (r *SnippetRepository) Update(s *models.Snippet) error {
	var expiresVal interface{}
	if s.ExpiresAt != nil {
		expiresVal = *s.ExpiresAt
	} else {
		expiresVal = nil
	}

	query := `UPDATE snippets SET title = $1, content = $2, language = $3, tags = $4, is_favorite = $5, expires_at = $6 WHERE id = $7`
	_, err := config.DB.Exec(query, s.Title, s.Content, s.Language, s.Tags, s.IsFavorite, expiresVal, s.ID)
	return err
}

func (r *SnippetRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM snippets WHERE id = $1`
	_, err := config.DB.Exec(query, id)
	return err
}

func (r *SnippetRepository) ToggleFavorite(id uuid.UUID, isFavorite bool) error {
	query := `UPDATE snippets SET is_favorite = $1 WHERE id = $2`
	_, err := config.DB.Exec(query, isFavorite, id)
	return err
}

// ==========================================
// SHARED FILE REPOSITORY
// ==========================================

type SharedFileRepository struct{}

func NewSharedFileRepository() *SharedFileRepository {
	return &SharedFileRepository{}
}

func (r *SharedFileRepository) Create(f *models.SharedFile) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	f.UploadedAt = time.Now()

	query := `INSERT INTO shared_files (id, file_name, file_path, file_size, file_type, sender_device, download_count, uploaded_at) 
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := config.DB.Exec(query, f.ID, f.FileName, f.FilePath, f.FileSize, f.FileType, f.SenderDevice, f.DownloadCount, f.UploadedAt)
	return err
}

func (r *SharedFileRepository) GetByID(id uuid.UUID) (*models.SharedFile, error) {
	f := &models.SharedFile{}
	query := `SELECT id, file_name, file_path, file_size, file_type, sender_device, download_count, uploaded_at FROM shared_files WHERE id = $1`
	err := config.DB.QueryRow(query, id).Scan(&f.ID, &f.FileName, &f.FilePath, &f.FileSize, &f.FileType, &f.SenderDevice, &f.DownloadCount, &f.UploadedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return f, nil
}

func (r *SharedFileRepository) GetAll(searchQuery string, fileType string) ([]models.SharedFile, error) {
	var list []models.SharedFile
	query := `SELECT id, file_name, file_path, file_size, file_type, sender_device, download_count, uploaded_at 
			  FROM shared_files 
			  WHERE (file_name ILIKE $1) `
	args := []interface{}{"%" + searchQuery + "%"}

	if fileType != "" {
		query += " AND file_type ILIKE $2"
		args = append(args, "%"+fileType+"%")
	}
	query += " ORDER BY uploaded_at DESC"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		f := models.SharedFile{}
		err := rows.Scan(&f.ID, &f.FileName, &f.FilePath, &f.FileSize, &f.FileType, &f.SenderDevice, &f.DownloadCount, &f.UploadedAt)
		if err != nil {
			return nil, err
		}
		list = append(list, f)
	}
	return list, nil
}

func (r *SharedFileRepository) IncrementDownloadCount(id uuid.UUID) error {
	query := `UPDATE shared_files SET download_count = download_count + 1 WHERE id = $1`
	_, err := config.DB.Exec(query, id)
	return err
}

func (r *SharedFileRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM shared_files WHERE id = $1`
	_, err := config.DB.Exec(query, id)
	return err
}

// ==========================================
// DEVICE REPOSITORY
// ==========================================

type DeviceRepository struct{}

func NewDeviceRepository() *DeviceRepository {
	return &DeviceRepository{}
}

func (r *DeviceRepository) Upsert(d *models.Device) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	d.LastSeen = time.Now()

	query := `INSERT INTO devices (id, device_name, ip_address, status, last_seen) 
			  VALUES ($1, $2, $3, $4, $5)
			  ON CONFLICT (ip_address) 
			  DO UPDATE SET device_name = EXCLUDED.device_name, status = EXCLUDED.status, last_seen = EXCLUDED.last_seen`
	_, err := config.DB.Exec(query, d.ID, d.DeviceName, d.IPAddress, d.Status, d.LastSeen)
	return err
}

func (r *DeviceRepository) GetAll() ([]models.Device, error) {
	var list []models.Device
	query := `SELECT id, device_name, ip_address, status, last_seen FROM devices ORDER BY last_seen DESC`
	rows, err := config.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		d := models.Device{}
		err := rows.Scan(&d.ID, &d.DeviceName, &d.IPAddress, &d.Status, &d.LastSeen)
		if err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, nil
}

func (r *DeviceRepository) UpdateStatus(ipAddress string, status string) error {
	query := `UPDATE devices SET status = $1, last_seen = $2 WHERE ip_address = $3`
	_, err := config.DB.Exec(query, status, time.Now(), ipAddress)
	return err
}

func (r *DeviceRepository) GetByIP(ipAddress string) (*models.Device, error) {
	d := &models.Device{}
	query := `SELECT id, device_name, ip_address, status, last_seen FROM devices WHERE ip_address = $1`
	err := config.DB.QueryRow(query, ipAddress).Scan(&d.ID, &d.DeviceName, &d.IPAddress, &d.Status, &d.LastSeen)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return d, nil
}

// ==========================================
// ACTIVITY LOG REPOSITORY
// ==========================================

type ActivityLogRepository struct{}

func NewActivityLogRepository() *ActivityLogRepository {
	return &ActivityLogRepository{}
}

func (r *ActivityLogRepository) Create(logVal *models.ActivityLog) error {
	if logVal.ID == uuid.Nil {
		logVal.ID = uuid.New()
	}
	logVal.CreatedAt = time.Now()

	var deviceIDVal interface{}
	if logVal.DeviceID != nil {
		deviceIDVal = *logVal.DeviceID
	} else {
		deviceIDVal = nil
	}

	query := `INSERT INTO activity_logs (id, activity_type, description, device_id, created_at) 
			  VALUES ($1, $2, $3, $4, $5)`
	_, err := config.DB.Exec(query, logVal.ID, logVal.ActivityType, logVal.Description, deviceIDVal, logVal.CreatedAt)
	return err
}

func (r *ActivityLogRepository) GetRecent(limit int) ([]models.ActivityLog, error) {
	var list []models.ActivityLog
	query := `SELECT id, activity_type, description, device_id, created_at FROM activity_logs ORDER BY created_at DESC LIMIT $1`
	rows, err := config.DB.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		l := models.ActivityLog{}
		var deviceIDVal uuid.NullUUID
		err := rows.Scan(&l.ID, &l.ActivityType, &l.Description, &deviceIDVal, &l.CreatedAt)
		if err != nil {
			return nil, err
		}
		if deviceIDVal.Valid {
			l.DeviceID = &deviceIDVal.UUID
		}
		list = append(list, l)
	}
	return list, nil
}
