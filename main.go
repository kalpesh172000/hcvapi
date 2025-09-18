package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/kalpesh172000/hcvapi/config"
	"github.com/kalpesh172000/hcvapi/handlers"
	"github.com/kalpesh172000/hcvapi/vault"
)

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.WithError(err).Fatal("Failed to load configuration")
	}

	logger.WithFields(logrus.Fields{
		"vault_address": cfg.Vault.Address,
		"server_port":   cfg.Server.Port,
		"gcp_project":   cfg.GCP.ProjectID,
	}).Info("Configuration loaded successfully")

	// Initialize Vault client
	vaultClient, err := vault.NewClient(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create Vault client")
	}

	// Initialize Vault GCP secrets engine
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := vaultClient.Initialize(ctx); err != nil {
		logger.WithError(err).Fatal("Failed to initialize Vault GCP secrets engine")
	}

	// Perform initial health check
	if err := vaultClient.HealthCheck(ctx); err != nil {
		logger.WithError(err).Fatal("Initial Vault health check failed")
	}

	// Initialize handlers
	handler := handlers.NewHandler(vaultClient, logger)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Add middlewares
	router.Use(handler.ErrorHandlingMiddleware())
	router.Use(handler.LoggingMiddleware())

	// Setup routes
	setupRoutes(router, handler)

	// Start server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.WithField("address", server.Addr).Info("Starting server...")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	logger.Info("Server started successfully. Press Ctrl+C to shutdown...")

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Create a context with timeout for graceful shutdown
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown the server gracefully
	if err := server.Shutdown(ctx); err != nil {
		logger.WithError(err).Fatal("Server forced to shutdown")
	}

	logger.Info("Server shutdown completed")
}

func setupRoutes(router *gin.Engine, handler *handlers.Handler) {
	// Health check
	router.GET("/health", handler.HealthCheck)

	// API v1 group
	v1 := router.Group("/api/v1")
	{
		// Roleset management
		rolesets := v1.Group("/rolesets")
		{
			rolesets.GET("", handler.ListRolesets)                    // GET /api/v1/rolesets
			rolesets.POST("/:name", handler.CreateRoleset)            // POST /api/v1/rolesets/{name}
			rolesets.DELETE("/:name", handler.DeleteRoleset)          // DELETE /api/v1/rolesets/{name}
		}

		// Token generation
		tokens := v1.Group("/tokens")
		{
			tokens.POST("/:name", handler.GetAccessToken)             // POST /api/v1/tokens/{name}
		}

		// Service account key generation
		keys := v1.Group("/keys")
		{
			keys.POST("/:name", handler.GetServiceAccountKey)         // POST /api/v1/keys/{name}
		}
	}
}
