package controllers

import (
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

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

// ==========================================
// REGISTER
// ==========================================

type RegisterInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// validateRegisterInput performs custom validation with clean error messages
func validateRegisterInput(input *RegisterInput) (string, bool) {
	// --- Username ---
	input.Username = strings.TrimSpace(input.Username)
	if input.Username == "" {
		return "Username is required", false
	}
	if len(input.Username) < 3 {
		return "Username must be at least 3 characters", false
	}
	if len(input.Username) > 30 {
		return "Username must not exceed 30 characters", false
	}
	// Only allow alphanumeric, underscores, and hyphens
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !usernameRegex.MatchString(input.Username) {
		return "Username can only contain letters, numbers, underscores, and hyphens", false
	}

	// --- Email ---
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	if input.Email == "" {
		return "Email is required", false
	}
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(input.Email) {
		return "Please enter a valid email address", false
	}

	// --- Password ---
	if input.Password == "" {
		return "Password is required", false
	}
	if len(input.Password) < 6 {
		return "Password must be at least 6 characters", false
	}
	if len(input.Password) > 15 {
		return "Password must not exceed 15 characters", false
	}

	// Password strength: at least one letter and one digit
	hasLetter := false
	hasDigit := false
	for _, ch := range input.Password {
		if unicode.IsLetter(ch) {
			hasLetter = true
		}
		if unicode.IsDigit(ch) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return "Password must contain at least one letter and one number", false
	}

	return "", true
}

func (ctrl *AuthController) Register(c *gin.Context) {
	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request. Please provide username, email, and password."})
		return
	}

	// Custom validation with clean error messages
	if errMsg, ok := validateRegisterInput(&input); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	// Check if username already exists
	existingUser, err := ctrl.userRepo.GetByUsername(input.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Something went wrong. Please try again."})
		return
	}
	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "This username is already taken. Please choose another one."})
		return
	}

	// Check if email already exists
	existingEmail, err := ctrl.userRepo.GetByEmail(input.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Something went wrong. Please try again."})
		return
	}
	if existingEmail != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "This email is already registered. Try logging in instead."})
		return
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(input.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to secure your account. Please try again."})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create your account. Please try again."})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Account created but failed to sign in. Please try logging in."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Welcome to DevSync! Your account has been created.",
		"token":   token,
		"user":    newUser,
	})
}

// ==========================================
// LOGIN
// ==========================================

type LoginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// validateLoginInput performs custom validation with clean error messages
func validateLoginInput(input *LoginInput) (string, bool) {
	input.Username = strings.TrimSpace(input.Username)
	if input.Username == "" {
		return "Username or email is required", false
	}

	if input.Password == "" {
		return "Password is required", false
	}
	if len(input.Password) < 6 {
		return "Password must be at least 6 characters", false
	}
	if len(input.Password) > 15 {
		return "Password must not exceed 15 characters", false
	}

	return "", true
}

func (ctrl *AuthController) Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request. Please provide username/email and password."})
		return
	}

	// Custom validation with clean error messages
	if errMsg, ok := validateLoginInput(&input); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	// Check user by username first, then fall back to email lookup
	user, err := ctrl.userRepo.GetByUsername(input.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Something went wrong. Please try again."})
		return
	}

	if user == nil {
		// Try email lookup
		user, err = ctrl.userRepo.GetByEmail(input.Username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Something went wrong. Please try again."})
			return
		}
	}

	if user == nil || !utils.CheckPasswordHash(input.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials. Please check your username and password."})
		return
	}

	// Generate token
	token, err := utils.GenerateToken(user.ID.String(), user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Login successful but failed to create session. Please try again."})
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
		"message": "Welcome back, " + user.Username + "!",
		"token":   token,
		"user":    user,
	})
}
