package controllers

import (
	"net/http"
	"time"

	"devsync/backend/models"
	"devsync/backend/repository"
	"devsync/backend/utils"
	"devsync/backend/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthController struct {
	userRepo     *repository.UserRepository
	activityRepo *repository.ActivityLogRepository
}

func NewAuthController(uRepo *repository.UserRepository, actRepo *repository.ActivityLogRepository) *AuthController {
	return &AuthController{
		userRepo:     uRepo,
		activityRepo: actRepo,
	}
}

type RegisterInput struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

func (ctrl *AuthController) Register(c *gin.Context) {
	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if username already exists
	existingUser, err := ctrl.userRepo.GetByUsername(input.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query database"})
		return
	}
	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already taken"})
		return
	}

	// Check if email already exists
	existingEmail, err := ctrl.userRepo.GetByEmail(input.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query database"})
		return
	}
	if existingEmail != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already in use"})
		return
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(input.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to secure password"})
		return
	}

	newUser := &models.User{
		ID:           uuid.New(),
		Username:     input.Username,
		Email:        input.Email,
		PasswordHash: hashedPassword,
		CreatedAt:    time.Now(),
	}

	if err := ctrl.userRepo.Create(newUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Log activity
	logEntry := &models.ActivityLog{
		ID:           uuid.New(),
		ActivityType: "auth",
		Description:  "New user registered: " + newUser.Username,
		CreatedAt:    time.Now(),
	}
	_ = ctrl.activityRepo.Create(logEntry)
	websocket.GlobalHub.Broadcast("activity", logEntry)

	// Generate JWT
	token, err := utils.GenerateToken(newUser.ID.String(), newUser.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate session token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Registration successful",
		"token":   token,
		"user":    newUser,
	})
}

type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (ctrl *AuthController) Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check user by username first, then check by email if username matches email format
	user, err := ctrl.userRepo.GetByUsername(input.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if user == nil {
		// Try email
		user, err = ctrl.userRepo.GetByEmail(input.Username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
	}

	if user == nil || !utils.CheckPasswordHash(input.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	// Generate token
	token, err := utils.GenerateToken(user.ID.String(), user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Log login activity
	logEntry := &models.ActivityLog{
		ID:           uuid.New(),
		ActivityType: "auth",
		Description:  "User logged in: " + user.Username,
		CreatedAt:    time.Now(),
	}
	_ = ctrl.activityRepo.Create(logEntry)
	websocket.GlobalHub.Broadcast("activity", logEntry)

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"token":   token,
		"user":    user,
	})
}
