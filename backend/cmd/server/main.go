package main

import (
	"clara-agents/internal/config"
	"clara-agents/internal/crypto"
	"clara-agents/internal/database"
	"clara-agents/internal/execution"
	"clara-agents/internal/handlers"
	"clara-agents/internal/logging"
	"clara-agents/internal/middleware"
	"clara-agents/internal/services"
	"clara-agents/internal/tools"
	"clara-agents/pkg/auth"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	logging.Init()
	log.Println("🚀 Starting Clara Agents backend…")

	if err := godotenv.Load(); err != nil {
		log.Printf("⚠️  No .env file: %v", err)
	}

	cfg := config.Load()
	log.Printf("📋 Config loaded (port: %s)", cfg.Port)

	// ── MySQL (providers, models) ─────────────────────────────────────────────
	if cfg.DatabaseURL == "" {
		log.Fatal("❌ DATABASE_URL is required")
	}
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("❌ MySQL: %v", err)
	}
	defer db.Close()
	if err := db.Initialize(); err != nil {
		log.Fatalf("❌ MySQL init: %v", err)
	}

	// ── MongoDB (agents, workflows, executions, conversations) ────────────────
	var mongoDB *database.MongoDB
	var encryptionService *crypto.EncryptionService
	var builderConvService *services.BuilderConversationService

	if cfg.MongoDBURI != "" {
		mongoDB, err = database.NewMongoDB(cfg.MongoDBURI)
		if err != nil {
			log.Printf("⚠️  MongoDB unavailable: %v", err)
		} else {
			log.Println("✅ MongoDB connected")
		}
	}

	if masterKey := os.Getenv("ENCRYPTION_MASTER_KEY"); masterKey != "" && mongoDB != nil {
		encryptionService, err = crypto.NewEncryptionService(masterKey)
		if err != nil {
			log.Printf("⚠️  Encryption init: %v", err)
		} else if encryptionService != nil {
			builderConvService = services.NewBuilderConversationService(mongoDB, encryptionService)
		}
	}

	// ── Core services ─────────────────────────────────────────────────────────
	providerService := services.NewProviderService(db)
	modelService := services.NewModelService(db)

	var credentialService *services.CredentialService
	if mongoDB != nil && encryptionService != nil {
		credentialService = services.NewCredentialService(mongoDB, encryptionService)
		_ = credentialService.EnsureIndexes(context.Background())
	}

	toolService := services.NewToolService(tools.GetRegistry(), credentialService)

	// Chat service → workflow generator (LLM calls in builder)
	chatService := services.NewChatService(db, providerService, nil, toolService)
	workflowGeneratorService := services.NewWorkflowGeneratorService(db, providerService, chatService)
	workflowGeneratorV2Service := services.NewWorkflowGeneratorV2Service(db, providerService, chatService)

	// ── MongoDB-dependent services ────────────────────────────────────────────
	var agentService *services.AgentService
	var executionService *services.ExecutionService
	var apiKeyService *services.APIKeyService
	var webhookService *services.WebhookService

	if mongoDB != nil {
		agentService = services.NewAgentService(mongoDB)
		_ = agentService.EnsureIndexes(context.Background())

		executionService = services.NewExecutionService(mongoDB, nil)
		_ = executionService.EnsureIndexes(context.Background())

		apiKeyService = services.NewAPIKeyService(mongoDB, nil)
		_ = apiKeyService.EnsureIndexes(context.Background())

		webhookService = services.NewWebhookService(mongoDB)
	}

	// ── Redis (scheduler) ─────────────────────────────────────────────────────
	var redisService *services.RedisService
	var schedulerService *services.SchedulerService

	if cfg.RedisURL != "" {
		redisService, err = services.NewRedisService(cfg.RedisURL)
		if err != nil {
			log.Printf("⚠️  Redis unavailable: %v", err)
		} else {
			log.Println("✅ Redis connected")
			if mongoDB != nil && agentService != nil && executionService != nil {
				schedulerService, err = services.NewSchedulerService(mongoDB, redisService, agentService, executionService)
				if err != nil {
					log.Printf("⚠️  Scheduler init: %v", err)
				}
			}
		}
	}
	_ = redisService

	// ── Execution engine ──────────────────────────────────────────────────────
	var workflowEngine *execution.WorkflowEngine
	executorRegistry := execution.NewExecutorRegistry(chatService, providerService, tools.GetRegistry(), credentialService)
	workflowEngine = execution.NewWorkflowEngineWithChecker(executorRegistry, providerService)

	// ── JWT auth ──────────────────────────────────────────────────────────────
	var jwtAuth *auth.LocalJWTAuth
	if cfg.JWTSecret != "" {
		jwtAuth, err = auth.NewLocalJWTAuth(cfg.JWTSecret, cfg.JWTAccessTokenExpiry, cfg.JWTRefreshTokenExpiry)
		if err != nil {
			log.Fatalf("❌ JWT auth init: %v", err)
		}
		log.Println("✅ JWT auth ready")
	} else {
		log.Println("⚠️  JWT_SECRET not set — auth disabled in dev mode")
	}

	// ── User service ──────────────────────────────────────────────────────────
	userService := services.NewUserService(mongoDB, cfg, nil)

	// ── Fiber app ─────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
		BodyLimit:    100 * 1024 * 1024, // 100 MB
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	})

	app.Use(recover.New())
	app.Use(logger.New())

	// ── Prometheus metrics (/metrics) ─────────────────────────────────────────
	prom := fiberprometheus.New("clara_agents")
	prom.RegisterAt(app, "/metrics")
	app.Use(prom.Middleware)

	// ── CORS ──────────────────────────────────────────────────────────────────
	allowedOrigins := cfg.AllowedOrigins
	if allowedOrigins == "" {
		allowedOrigins = "*"
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-API-Key",
		AllowCredentials: !strings.Contains(allowedOrigins, "*"),
		MaxAge:           86400,
	}))

	// ── Health ────────────────────────────────────────────────────────────────
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
		})
	})

	// ── Auth middleware + routes ──────────────────────────────────────────────
	jwtMw := middleware.LocalAuthMiddleware(jwtAuth)
	optionalMw := middleware.OptionalLocalAuthMiddleware(jwtAuth)
	_ = optionalMw

	authHandler := handlers.NewLocalAuthHandler(jwtAuth, userService)
	authGroup := app.Group("/api/auth")
	authGroup.Post("/register", authHandler.Register)
	authGroup.Post("/login", authHandler.Login)
	authGroup.Post("/refresh", authHandler.RefreshToken)
	authGroup.Post("/logout", jwtMw, authHandler.Logout)
	authGroup.Get("/me", jwtMw, authHandler.GetCurrentUser)

	// ── Agent routes ──────────────────────────────────────────────────────────
	agentHandler := handlers.NewAgentHandler(agentService, workflowGeneratorService)
	agentHandler.SetWorkflowGeneratorV2Service(workflowGeneratorV2Service)
	agentHandler.SetBuilderConversationService(builderConvService)
	agentHandler.SetProviderService(providerService)
	agentHandler.SetWebhookService(webhookService)
	agentHandler.SetSchedulerService(schedulerService)
	agentHandler.SetExecutorRegistry(executorRegistry)

	apiGroup := app.Group("/api", jwtMw)
	agentGroup := apiGroup.Group("/agents")
	agentGroup.Post("/", agentHandler.Create)
	agentGroup.Get("/", agentHandler.List)
	agentGroup.Get("/recent", agentHandler.ListRecent)
	agentGroup.Post("/autofill", agentHandler.AutoFillBlock)
	agentGroup.Post("/test-block", agentHandler.TestBlock)
	agentGroup.Post("/ask", agentHandler.Ask)
	agentGroup.Get("/:id", agentHandler.Get)
	agentGroup.Put("/:id", agentHandler.Update)
	agentGroup.Delete("/:id", agentHandler.Delete)
	agentGroup.Get("/:id/workflow", agentHandler.GetWorkflow)
	agentGroup.Put("/:id/workflow", agentHandler.SaveWorkflow)
	agentGroup.Get("/:id/workflow/versions", agentHandler.ListWorkflowVersions)
	agentGroup.Get("/:id/workflow/versions/:version", agentHandler.GetWorkflowVersion)
	agentGroup.Post("/:id/workflow/versions/:version/restore", agentHandler.RestoreWorkflowVersion)
	agentGroup.Post("/:id/generate-workflow", agentHandler.GenerateWorkflow)
	agentGroup.Post("/:id/generate-workflow-v2", agentHandler.GenerateWorkflowV2)
	agentGroup.Post("/:id/select-tools", agentHandler.SelectTools)
	agentGroup.Post("/:id/generate-with-tools", agentHandler.GenerateWithTools)
	agentGroup.Post("/:id/sync", agentHandler.SyncAgent)
	agentGroup.Get("/:id/webhook", agentHandler.GetWebhook)
	agentGroup.Delete("/:id/webhook", agentHandler.DeleteWebhook)

	// ── WebSocket (workflow execution streaming) ──────────────────────────────
	wsHandler := handlers.NewWorkflowWebSocketHandler(agentService, workflowEngine, nil)
	wsHandler.SetExecutionService(executionService)

	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws/workflow", websocket.New(wsHandler.Handle))

	// ── Trigger routes ────────────────────────────────────────────────────────
	triggerHandler := handlers.NewTriggerHandler(agentService, executionService, workflowEngine)
	triggerGroup := app.Group("/api/trigger")
	triggerGroup.Post("/:agentId", triggerHandler.TriggerAgent)
	triggerGroup.Get("/:agentId/status/:executionId", triggerHandler.GetExecutionStatus)

	// Schedule usage stats for the current user
	if schedulerService != nil {
		apiGroup.Get("/schedules/usage", func(c *fiber.Ctx) error {
			userID, ok := c.Locals("user_id").(string)
			if !ok || userID == "" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
			}
			usage, err := schedulerService.GetScheduleUsage(c.Context(), userID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(usage)
		})
	} else {
		// Scheduler unavailable (no Redis) — return sensible defaults
		apiGroup.Get("/schedules/usage", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{
				"active": 0, "paused": 0, "total": 0, "limit": 5, "canCreate": true,
			})
		})
	}

	// Webhook inbound (no auth — secret validated inside)
	if webhookService != nil {
		app.Post("/api/webhook/:agentId", triggerHandler.TriggerAgent)
	}

	// ── Model + Provider routes (for model selector in builder) ───────────────
	modelHandler := handlers.NewModelHandler(modelService)
	apiGroup.Get("/models", modelHandler.List)
	apiGroup.Get("/providers", func(c *fiber.Ctx) error {
		providers, err := providerService.GetAll()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch providers"})
		}
		return c.JSON(providers)
	})

	// ── Admin routes (provider + model management) ────────────────────────────
	modelMgmtService := services.NewModelManagementService(db)
	adminHnd := handlers.NewAdminHandler(providerService, modelService, modelMgmtService)
	modelMgmtHnd := handlers.NewModelManagementHandler(modelMgmtService, modelService, providerService)
	adminMw := middleware.AdminMiddleware(cfg)
	adminGroup := app.Group("/api/admin", jwtMw, adminMw)
	adminGroup.Get("/me", adminHnd.GetAdminStatus)
	// Provider CRUD
	adminGroup.Get("/providers", adminHnd.GetProviders)
	adminGroup.Post("/providers", adminHnd.CreateProvider)
	adminGroup.Put("/providers/:id", adminHnd.UpdateProvider)
	adminGroup.Delete("/providers/:id", adminHnd.DeleteProvider)
	adminGroup.Put("/providers/:id/toggle", adminHnd.ToggleProvider)
	adminGroup.Post("/providers/:id/fetch", adminHnd.FetchModelsFromProvider)
	adminGroup.Post("/providers/:id/sync", adminHnd.SyncProviderToJSON)
	// Model CRUD + operations
	adminGroup.Get("/models", modelMgmtHnd.GetAllModels)
	adminGroup.Post("/models", modelMgmtHnd.CreateModel)
	adminGroup.Put("/models/by-id", modelMgmtHnd.UpdateModel)
	adminGroup.Delete("/models/by-id", modelMgmtHnd.DeleteModel)
	adminGroup.Post("/models/by-id/test/connection", modelMgmtHnd.TestModelConnection)
	adminGroup.Post("/models/by-id/test/capability", modelMgmtHnd.TestModelCapability)
	adminGroup.Post("/models/by-id/benchmark", modelMgmtHnd.RunModelBenchmark)
	adminGroup.Get("/models/by-id/test-results", modelMgmtHnd.GetModelTestResults)
	adminGroup.Get("/models/by-id/aliases", modelMgmtHnd.GetModelAliases)
	adminGroup.Post("/models/by-id/aliases", modelMgmtHnd.CreateModelAlias)
	adminGroup.Put("/models/by-id/aliases/:alias", modelMgmtHnd.UpdateModelAlias)
	adminGroup.Delete("/models/by-id/aliases/:alias", modelMgmtHnd.DeleteModelAlias)
	adminGroup.Post("/models/import-aliases", modelMgmtHnd.ImportAliasesFromJSON)
	adminGroup.Put("/models/bulk/agents-enabled", modelMgmtHnd.BulkUpdateAgentsEnabled)
	adminGroup.Put("/models/bulk/visibility", modelMgmtHnd.BulkUpdateVisibility)
	adminGroup.Put("/models/bulk/tier", modelMgmtHnd.BulkUpdateTier)
	adminGroup.Post("/models/by-id/tier", modelMgmtHnd.SetModelTier)
	adminGroup.Delete("/models/by-id/tier", modelMgmtHnd.ClearModelTier)
	adminGroup.Get("/tiers", modelMgmtHnd.GetTiers)

	// ── Credential routes ─────────────────────────────────────────────────────
	if credentialService != nil {
		credHnd := handlers.NewCredentialHandler(credentialService)
		credGroup := apiGroup.Group("/credentials")
		credGroup.Get("/", credHnd.List)
		credGroup.Post("/", credHnd.Create)
		credGroup.Get("/:id", credHnd.Get)
		credGroup.Put("/:id", credHnd.Update)
		credGroup.Delete("/:id", credHnd.Delete)
		credGroup.Post("/:id/test", credHnd.Test)
		credGroup.Get("/references", credHnd.GetCredentialReferences)
		apiGroup.Get("/integrations", credHnd.GetIntegrations)
	}

	// ── Upload route ──────────────────────────────────────────────────────────
	uploadDir := cfg.UploadDir
	if uploadDir == "" {
		uploadDir = "/app/uploads"
	}
	uploadHnd := handlers.NewUploadHandler(uploadDir, nil)
	apiGroup.Post("/upload", uploadHnd.Upload)

	// ── API key routes (for deployed agents) ─────────────────────────────────
	if apiKeyService != nil {
		app.Post("/api/agent/:agentId/invoke", middleware.APIKeyMiddleware(apiKeyService), triggerHandler.TriggerAgent)
	}

	// ── Tools route ───────────────────────────────────────────────────────────
	toolsHnd := handlers.NewToolsHandler(tools.GetRegistry(), toolService)
	apiGroup.Get("/tools", toolsHnd.ListTools)

	// ── Start scheduler ───────────────────────────────────────────────────────
	if schedulerService != nil {
		go func() {
			log.Println("⏰ Starting scheduler service…")
			if err := schedulerService.Start(context.Background()); err != nil {
				log.Printf("⚠️  Scheduler error: %v", err)
			}
		}()
	}

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		addr := fmt.Sprintf(":%s", cfg.Port)
		log.Printf("🌐 Listening on %s", addr)
		if err := app.Listen(addr); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	<-quit
	log.Println("🛑 Shutting down…")
	if err := app.ShutdownWithTimeout(15 * time.Second); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
	log.Println("👋 Goodbye")
}
