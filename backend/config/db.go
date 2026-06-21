package config

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() {
	var err error
	log.Println("Connecting to database...")

	// Retry connection a few times (helps when docker container is starting up)
	for i := 0; i < 5; i++ {
		DB, err = sql.Open("postgres", AppConfig.DBConnStr)
		if err == nil {
			err = DB.Ping()
		}
		if err == nil {
			break
		}
		log.Printf("Database connection attempt %d failed: %v. Retrying in 3 seconds...", i+1, err)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		log.Fatalf("Could not connect to database after retries: %v", err)
	}

	log.Println("Database connection established successfully!")

	// Create tables if they do not exist
	runMigrations()
}

func runMigrations() {
	schemas := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			username VARCHAR(100) UNIQUE NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS snippets (
			id UUID PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			content TEXT NOT NULL,
			language VARCHAR(100) NOT NULL,
			tags TEXT,
			is_favorite BOOLEAN DEFAULT FALSE,
			expires_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS shared_files (
			id UUID PRIMARY KEY,
			file_name VARCHAR(255) NOT NULL,
			file_path VARCHAR(512) NOT NULL,
			file_size BIGINT NOT NULL,
			file_type VARCHAR(100),
			sender_device VARCHAR(255),
			download_count INTEGER DEFAULT 0,
			relative_path VARCHAR(512) DEFAULT '',
			uploaded_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS devices (
			id UUID PRIMARY KEY,
			device_name VARCHAR(255),
			ip_address VARCHAR(100) UNIQUE NOT NULL,
			status VARCHAR(50) DEFAULT 'online',
			last_seen TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS activity_logs (
			id UUID PRIMARY KEY,
			activity_type VARCHAR(100) NOT NULL,
			description TEXT NOT NULL,
			device_id UUID REFERENCES devices(id) ON DELETE SET NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, schema := range schemas {
		_, err := DB.Exec(schema)
		if err != nil {
			log.Fatalf("Error running migration: %v\nSchema: %s", err, schema)
		}
	}

	// Upgrade script to ensure relative_path exists for older database containers
	_, err := DB.Exec("ALTER TABLE shared_files ADD COLUMN IF NOT EXISTS relative_path VARCHAR(512) DEFAULT '';")
	if err != nil {
		log.Printf("Warning: Failed to run upgrade migration: %v", err)
	}

	log.Println("Database tables initialized successfully!")
}
