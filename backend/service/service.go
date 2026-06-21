package service

import (
	"log"

	"devsync/backend/config"
	"devsync/backend/controllers"
	"devsync/backend/repository"
	"devsync/backend/routes"
	"devsync/backend/websocket"
	"devsync/backend/worker"
)

func Server() {
	// 1. Load Configurations
	config.LoadConfig()

	// 2. Connect to Database & Run migrations
	config.InitDB()

	// 3. Initialize WebSocket Hub
	websocket.InitHub()

	// 4. Initialize Repositories (Raw SQL)
	userRepo := repository.NewUserRepository()
	snippetRepo := repository.NewSnippetRepository()
	fileRepo := repository.NewSharedFileRepository()
	deviceRepo := repository.NewDeviceRepository()
	activityRepo := repository.NewActivityLogRepository()

	// Start Background Workers
	worker.StartDeviceOfflineWorker(deviceRepo)

	// 5. Initialize Controllers
	authCtrl := controllers.NewAuthController(userRepo, activityRepo)
	snippetCtrl := controllers.NewSnippetController(snippetRepo, activityRepo)
	fileCtrl := controllers.NewFileController(fileRepo, activityRepo)
	deviceCtrl := controllers.NewDeviceController(deviceRepo, activityRepo)
	dashCtrl := controllers.NewDashboardController(fileRepo, snippetRepo, deviceRepo, activityRepo)

	// 6. Setup Router & Routes
	router := routes.SetupRouter(authCtrl, snippetCtrl, fileCtrl, deviceCtrl, dashCtrl)

	// 7. Start Server (Binding to 0.0.0.0)
	addr := "0.0.0.0:" + config.AppConfig.Port
	log.Printf("DevSync Server starting on %s...", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
