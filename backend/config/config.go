package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DBConnStr   string
	JWTSecret   string
	UploadDir   string
}

var AppConfig Config

func LoadConfig() {
	// Try loading .env. If it fails, rely on system env vars
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment variables")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "postgrespassword"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "devsync"
	}
	dbSSLMode := os.Getenv("DB_SSLMODE")
	if dbSSLMode == "" {
		dbSSLMode = "disable"
	}

	connStr := "host=" + dbHost + " port=" + dbPort + " user=" + dbUser + " password=" + dbPassword + " dbname=" + dbName + " sslmode=" + dbSSLMode

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "devsync-default-secret"
	}

	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}

	// Ensure upload directory exists
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory %s: %v", uploadDir, err)
	}

	// Create chunks temp directory inside uploadDir
	if err := os.MkdirAll(uploadDir+"/temp_chunks", 0755); err != nil {
		log.Fatalf("Failed to create chunks temp directory: %v", err)
	}

	AppConfig = Config{
		Port:        port,
		DBConnStr:   connStr,
		JWTSecret:   jwtSecret,
		UploadDir:   uploadDir,
	}
}
