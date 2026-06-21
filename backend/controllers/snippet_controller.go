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

type SnippetController struct {
	snippetRepo  *repository.SnippetRepository
	activityRepo *repository.ActivityLogRepository
}

func NewSnippetController(sRepo *repository.SnippetRepository, actRepo *repository.ActivityLogRepository) *SnippetController {
	return &SnippetController{
		snippetRepo:  sRepo,
		activityRepo: actRepo,
	}
}

type SnippetInput struct {
	Title      string     `json:"title" binding:"required"`
	Content    string     `json:"content" binding:"required"`
	Language   string     `json:"language" binding:"required"`
	Tags       string     `json:"tags"`
	IsFavorite bool       `json:"is_favorite"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

func (ctrl *SnippetController) Create(c *gin.Context) {
	var input SnippetInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	snippet := &models.Snippet{
		ID:         uuid.New(),
		Title:      input.Title,
		Content:    input.Content,
		Language:   input.Language,
		Tags:       input.Tags,
		IsFavorite: input.IsFavorite,
		ExpiresAt:  input.ExpiresAt,
		CreatedAt:  time.Now(),
	}

	if err := ctrl.snippetRepo.Create(snippet); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save snippet"})
		return
	}

	// Log activity
	username, _ := c.Get("username")
	logEntry := &models.ActivityLog{
		ID:           uuid.New(),
		ActivityType: "snippet_create",
		Description:  "User " + username.(string) + " created snippet: " + snippet.Title,
		CreatedAt:    time.Now(),
	}
	_ = ctrl.activityRepo.Create(logEntry)
	
	// Real-time broadcasts
	websocket.GlobalHub.Broadcast("new_snippet", snippet)
	websocket.GlobalHub.Broadcast("activity", logEntry)

	c.JSON(http.StatusCreated, snippet)
}

func (ctrl *SnippetController) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid UUID format"})
		return
	}

	snippet, err := ctrl.snippetRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch snippet"})
		return
	}
	if snippet == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Snippet not found"})
		return
	}

	c.JSON(http.StatusOK, snippet)
}

func (ctrl *SnippetController) List(c *gin.Context) {
	searchQuery := c.Query("q")
	language := c.Query("language")

	list, err := ctrl.snippetRepo.GetAll(searchQuery, language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list snippets"})
		return
	}

	c.JSON(http.StatusOK, list)
}

func (ctrl *SnippetController) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid UUID format"})
		return
	}

	var input SnippetInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	snippet, err := ctrl.snippetRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if snippet == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Snippet not found"})
		return
	}

	snippet.Title = input.Title
	snippet.Content = input.Content
	snippet.Language = input.Language
	snippet.Tags = input.Tags
	snippet.IsFavorite = input.IsFavorite
	snippet.ExpiresAt = input.ExpiresAt

	if err := ctrl.snippetRepo.Update(snippet); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update snippet"})
		return
	}

	// Log activity
	username, _ := c.Get("username")
	logEntry := &models.ActivityLog{
		ID:           uuid.New(),
		ActivityType: "snippet_update",
		Description:  "User " + username.(string) + " updated snippet: " + snippet.Title,
		CreatedAt:    time.Now(),
	}
	_ = ctrl.activityRepo.Create(logEntry)
	
	websocket.GlobalHub.Broadcast("snippet_update", snippet)
	websocket.GlobalHub.Broadcast("activity", logEntry)

	c.JSON(http.StatusOK, snippet)
}

func (ctrl *SnippetController) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid UUID format"})
		return
	}

	snippet, err := ctrl.snippetRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if snippet == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Snippet not found"})
		return
	}

	if err := ctrl.snippetRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete snippet"})
		return
	}

	// Log activity
	username, _ := c.Get("username")
	logEntry := &models.ActivityLog{
		ID:           uuid.New(),
		ActivityType: "snippet_delete",
		Description:  "User " + username.(string) + " deleted snippet: " + snippet.Title,
		CreatedAt:    time.Now(),
	}
	_ = ctrl.activityRepo.Create(logEntry)
	
	websocket.GlobalHub.Broadcast("snippet_delete", idStr)
	websocket.GlobalHub.Broadcast("activity", logEntry)

	c.JSON(http.StatusOK, gin.H{"message": "Snippet deleted successfully"})
}

type ToggleFavInput struct {
	IsFavorite bool `json:"is_favorite"`
}

func (ctrl *SnippetController) ToggleFavorite(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid UUID format"})
		return
	}

	var input ToggleFavInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	snippet, err := ctrl.snippetRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if snippet == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Snippet not found"})
		return
	}

	if err := ctrl.snippetRepo.ToggleFavorite(id, input.IsFavorite); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update favorite status"})
		return
	}

	snippet.IsFavorite = input.IsFavorite
	websocket.GlobalHub.Broadcast("snippet_update", snippet)

	c.JSON(http.StatusOK, snippet)
}
