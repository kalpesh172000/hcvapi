package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kalpesh172000/hcvapi/vault"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	vaultClient *vault.Client
	logger      *logrus.Logger
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type TokenRequest struct {
	TTL string `json:"ttl,omitempty"`
}

func NewHandler(vaultClient *vault.Client, logger *logrus.Logger) *Handler {
	return &Handler{
		vaultClient: vaultClient,
		logger:      logger,
	}
}

// Health check endpoint
func (h *Handler) HealthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := h.vaultClient.HealthCheck(ctx); err != nil {
		h.logger.WithError(err).Error("Health check failed")
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error:   "Service unavailable",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Service is healthy",
	})
}

// Create a new roleset
func (h *Handler) CreateRoleset(c *gin.Context) {
	rolesetName := c.Param("name")
	if rolesetName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Roleset name required"})
		return
	}

	var req vault.RolesetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert the map in JSON input to string
	bindingsMap := make(map[string]interface{})
	if err := json.Unmarshal([]byte(req.Bindings), &bindingsMap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid bindings format"})
		return
	}

	bindingsJSON, _ := json.Marshal(bindingsMap)
	req.Bindings = string(bindingsJSON)

	if err := h.vaultClient.CreateRoleset(context.Background(), rolesetName, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Roleset created successfully"})
}

// Generate access token
func (h *Handler) GetAccessToken(c *gin.Context) {
	rolesetName := c.Param("name")
	if rolesetName == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Roleset name is required",
		})
		return
	}

	var tokenReq TokenRequest
	// TTL is optional, so ignore bind errors
	_ = c.ShouldBindJSON(&tokenReq)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	token, err := h.vaultClient.GetToken(ctx, rolesetName, tokenReq.TTL)
	if err != nil {
		h.logger.WithError(err).WithField("roleset", rolesetName).Error("Failed to get access token")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to generate access token",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Access token generated successfully",
		Data:    token,
	})
}

// Generate service account key
func (h *Handler) GetServiceAccountKey(c *gin.Context) {
	rolesetName := c.Param("name")
	if rolesetName == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Roleset name is required",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	key, err := h.vaultClient.GetServiceAccountKey(ctx, rolesetName)
	if err != nil {
		h.logger.WithError(err).WithField("roleset", rolesetName).Error("Failed to get service account key")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to generate service account key",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Service account key generated successfully",
		Data:    key,
	})
}

// List all rolesets
func (h *Handler) ListRolesets(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	rolesets, err := h.vaultClient.ListRolesets(ctx)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list rolesets")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to list rolesets",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Rolesets retrieved successfully",
		Data: map[string]interface{}{
			"rolesets": rolesets,
			"count":    len(rolesets),
		},
	})
}

// Delete a roleset
func (h *Handler) DeleteRoleset(c *gin.Context) {
	rolesetName := c.Param("name")
	if rolesetName == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Roleset name is required",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	if err := h.vaultClient.DeleteRoleset(ctx, rolesetName); err != nil {
		h.logger.WithError(err).WithField("roleset", rolesetName).Error("Failed to delete roleset")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete roleset",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Roleset deleted successfully",
		Data: map[string]string{
			"name": rolesetName,
		},
	})
}

// Middleware for logging requests
func (h *Handler) LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate request duration
		duration := time.Since(start)

		// Build log entry
		entry := h.logger.WithFields(logrus.Fields{
			"status":     c.Writer.Status(),
			"method":     c.Request.Method,
			"path":       path,
			"query":      raw,
			"ip":         c.ClientIP(),
			"user-agent": c.Request.UserAgent(),
			"duration":   duration,
		})

		if len(c.Errors) > 0 {
			entry.Error(c.Errors.String())
		} else {
			entry.Info("Request completed")
		}
	}
}

// Middleware for error handling
func (h *Handler) ErrorHandlingMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		h.logger.WithField("panic", recovered).Error("Request panic recovered")

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Internal server error",
		})
	})
}
