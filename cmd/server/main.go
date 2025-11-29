package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"smarticky/ent"
	"smarticky/internal/handler"
	"smarticky/internal/logger"
	authmw "smarticky/internal/middleware"
	"smarticky/internal/storage"
	"smarticky/internal/version"
	"smarticky/web"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib-x/entsqlite"
	"go.uber.org/zap"
)

func getDataDir() string {
	// Try to get data directory from environment variable
	dataDir := os.Getenv("SMARTICKY_DATA_DIR")
	if dataDir == "" {
		// Default to ./data directory
		dataDir = "data"
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Printf("Failed to create data directory: %v\n", err)
		os.Exit(1)
	}

	return dataDir
}

func getDatabasePath() string {
	dataDir := getDataDir()
	dbPath := filepath.Join(dataDir, "smarticky.db")

	// Return SQLite connection string with optimizations
	return fmt.Sprintf("file:%s?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)", dbPath)
}

func main() {
	// 1. Initialize data directory
	dataDir := getDataDir()

	// 2. Initialize logger
	if err := logger.InitLogger(dataDir); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	zap.L().Info("Starting Smarticky Notes",
		zap.String("data_dir", dataDir),
	)

	// 3. Initialize Ent client with configurable data directory
	dbPath := getDatabasePath()
	zap.L().Info("Using database", zap.String("path", dbPath))

	client, err := ent.Open("sqlite3", dbPath)
	if err != nil {
		zap.L().Fatal("Failed to open database connection", zap.Error(err))
	}
	defer client.Close()

	// Run the auto migration tool
	if err := client.Schema.Create(context.Background()); err != nil {
		zap.L().Warn("Schema migration failed, trying to continue", zap.Error(err))
	}

	// 4. Initialize Echo
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(middleware.RequestID())
	e.Use(zapLoggerMiddleware())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(middleware.Secure())
	e.Use(middleware.Gzip())

	// 5. Initialize FileSystem and Handlers
	fs := storage.NewFileSystem("")
	h := handler.NewHandler(client, fs)

	// Start automatic backup scheduler
	h.StartAutoBackup()

	// 4. Routes
	// API
	api := e.Group("/api")

	// Public routes (no auth required)
	api.GET("/setup/check", h.CheckSetup)
	api.POST("/setup", h.Setup)
	api.POST("/auth/login", h.Login)

	// Version info endpoint (public)
	api.GET("/version", func(c echo.Context) error {
		return c.JSON(http.StatusOK, version.GetInfo())
	})

	// Protected routes (auth required)
	protected := api.Group("")
	protected.Use(authmw.JWTAuth())

	// Auth endpoints
	protected.GET("/auth/me", h.GetCurrentUser)
	protected.POST("/auth/logout", h.Logout)

	// Notes API
	protected.GET("/notes", h.ListNotes)
	protected.POST("/notes", h.CreateNote)
	protected.GET("/notes/:id", h.GetNote)
	protected.PUT("/notes/:id", h.UpdateNote)
	protected.DELETE("/notes/:id", h.DeleteNote)
	protected.POST("/notes/:id/verify-password", h.VerifyNotePassword)

	// Attachments API
	protected.POST("/notes/:id/attachments", h.UploadAttachment)
	protected.GET("/notes/:id/attachments", h.ListAttachments)
	protected.GET("/attachments/:id/download", h.DownloadAttachment)
	protected.DELETE("/attachments/:id", h.DeleteAttachment)

	// User management (admin only for most)
	adminRoutes := protected.Group("/users")
	adminRoutes.Use(authmw.AdminOnly())
	adminRoutes.GET("", h.ListUsers)
	adminRoutes.POST("", h.CreateUser)
	adminRoutes.DELETE("/:id", h.DeleteUser)

	// User self-management (authenticated users can manage themselves)
	protected.PUT("/users/:id", h.UpdateUser)
	protected.PUT("/users/:id/password", h.UpdatePassword)
	protected.POST("/users/:id/avatar", h.UploadAvatar)

	// Backup Config API
	protected.GET("/backup/config", h.GetBackupConfig)
	protected.PUT("/backup/config", h.UpdateBackupConfig)

	// Backup & Restore API
	protected.POST("/backup/webdav", h.BackupWebDAV)
	protected.POST("/backup/s3", h.BackupS3)
	protected.POST("/restore/webdav", h.RestoreWebDAV)
	protected.POST("/restore/s3", h.RestoreS3)

	// Backup List API
	protected.GET("/backup/list/webdav", h.ListWebDAVBackups)
	protected.GET("/backup/list/s3", h.ListS3Backups)

	// Backup Verify API
	protected.POST("/backup/verify/webdav", h.VerifyWebDAVBackup)
	protected.POST("/backup/verify/s3", h.VerifyS3Backup)

	// Serve uploaded files from data directory
	uploadsDir := filepath.Join(getDataDir(), "uploads")
	e.Static("/uploads", uploadsDir)

	// Static Files - Use embedded FS
	webFS := echo.MustSubFS(web.Assets, "static")
	e.StaticFS("/static", webFS)

	// Frontend - Serve index.html from embedded FS
	e.GET("/", func(c echo.Context) error {
		htmlContent, err := web.Assets.ReadFile("templates/index.html")
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to load page")
		}
		return c.HTMLBlob(http.StatusOK, htmlContent)
	})

	// Setup page
	e.GET("/setup", func(c echo.Context) error {
		htmlContent, err := web.Assets.ReadFile("templates/setup.html")
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to load setup page")
		}
		return c.HTMLBlob(http.StatusOK, htmlContent)
	})

	// Login page
	e.GET("/login", func(c echo.Context) error {
		htmlContent, err := web.Assets.ReadFile("templates/login.html")
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to load login page")
		}
		return c.HTMLBlob(http.StatusOK, htmlContent)
	})

	// Test page
	e.GET("/test", func(c echo.Context) error {
		htmlContent, err := web.Assets.ReadFile("templates/test.html")
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to load test page")
		}
		return c.HTMLBlob(http.StatusOK, htmlContent)
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	zap.L().Info("Server starting", zap.String("port", port))
	if err := e.Start(":" + port); err != nil && err != http.ErrServerClosed {
		zap.L().Fatal("Server failed to start", zap.Error(err))
	}
}

// zapLoggerMiddleware returns a middleware that logs HTTP requests using zap
func zapLoggerMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			req := c.Request()
			res := c.Response()

			err := next(c)

			// Calculate duration
			duration := time.Since(start)

			// Log request details
			fields := []zap.Field{
				zap.String("method", req.Method),
				zap.String("uri", req.RequestURI),
				zap.Int("status", res.Status),
				zap.Int64("bytes_out", res.Size),
				zap.Duration("duration", duration),
				zap.String("remote_ip", c.RealIP()),
				zap.String("user_agent", req.UserAgent()),
			}

			// Add request ID if available
			if reqID := c.Response().Header().Get(echo.HeaderXRequestID); reqID != "" {
				fields = append(fields, zap.String("request_id", reqID))
			}

			// Log errors at error level, success at info level
			if err != nil {
				fields = append(fields, zap.Error(err))
				zap.L().Error("Request failed", fields...)
			} else if res.Status >= 500 {
				zap.L().Error("Server error", fields...)
			} else if res.Status >= 400 {
				zap.L().Warn("Client error", fields...)
			} else {
				zap.L().Info("Request completed", fields...)
			}

			return err
		}
	}
}
