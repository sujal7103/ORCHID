# Clara Agents — First-Time Setup & Missing Routes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the two critical first-time-setup blockers (MySQL absent from docker-compose, incomplete .env.example) and implement all 20+ missing backend HTTP routes the frontend calls.

**Architecture:** Infrastructure fixes are pure config changes. Missing routes are implemented by: (a) adding new service methods to `AnalyticsService` and `ModelManagementService`, (b) creating two new handler files (`execution.go`, `analytics_admin.go`), (c) adding methods to existing handlers, and (d) wiring all routes in `main.go`. Insights/Autopilot routes that have no service logic are stubbed with well-shaped empty responses so the frontend renders rather than crashing.

**Tech Stack:** Go 1.21+, Fiber v2, MongoDB (`go.mongodb.org/mongo-driver`), MySQL (`database/sql`), Docker Compose v2

---

## File Structure

**Create:**
- `backend/internal/handlers/execution.go` — HTTP handler for execution history (list by agent, list by user, get by ID)
- `backend/internal/handlers/analytics_admin.go` — HTTP handler for admin analytics, users/GDPR, health, insights (stubs), autopilot (stub)

**Modify:**
- `backend/internal/handlers/model_management.go` — add `BulkUpdateTier` method
- `backend/internal/handlers/tools.go` — add `ListRegistry` method
- `backend/internal/services/model_management_service.go` — add `BulkUpdateTier(modelIDs []string, tier string) error`
- `backend/internal/services/analytics_service.go` — add `GetModelAnalytics`, `GetCollectionStats`
- `backend/cmd/server/main.go` — wire new handlers/routes, add AnalyticsService init
- `docker-compose.yml` — add MySQL service + volume + backend dependency
- `.env.example` — add DATABASE_URL and complete all required vars

---

### Task 1: Add MySQL to docker-compose.yml

**Files:**
- Modify: `docker-compose.yml`

- [ ] **Step 1: Add the `mysql` service block**

In `docker-compose.yml`, insert this service **before** the `volumes:` section (after the `redis` service):

```yaml
  # ── MySQL (providers, models, schema) ─────────────────────────────────────
  mysql:
    image: mysql:8.0
    container_name: clara-agents-mysql
    environment:
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD:-rootpassword}
      MYSQL_DATABASE: ${MYSQL_DATABASE:-claraverse}
      MYSQL_USER: ${MYSQL_USER:-clara}
      MYSQL_PASSWORD: ${MYSQL_PASSWORD:-clarapassword}
    ports:
      - "${MYSQL_PORT:-3306}:3306"
    volumes:
      - mysql-data:/var/lib/mysql
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost", "-u", "root", "-p${MYSQL_ROOT_PASSWORD:-rootpassword}"]
      interval: 10s
      timeout: 5s
      start_period: 30s
      retries: 5
    networks:
      - clara-agents
```

- [ ] **Step 2: Add mysql-data to the volumes section**

In `docker-compose.yml` under `volumes:`, add:

```yaml
  mysql-data:
```

- [ ] **Step 3: Add mysql dependency to the backend service**

In `docker-compose.yml`, in the `backend:` service under `depends_on:`, add:

```yaml
      mysql:
        condition: service_healthy
```

- [ ] **Step 4: Add DATABASE_URL to backend environment**

In `docker-compose.yml`, in the `backend:` service under `environment:`, add:

```yaml
      - DATABASE_URL=mysql://${MYSQL_USER:-clara}:${MYSQL_PASSWORD:-clarapassword}@mysql:3306/${MYSQL_DATABASE:-claraverse}?parseTime=true
```

- [ ] **Step 5: Validate the compose file**

Run: `docker compose config --quiet`
Expected: No error output (exits 0).

- [ ] **Step 6: Commit**

```bash
git add docker-compose.yml
git commit -m "fix: add MySQL service to docker-compose and wire DATABASE_URL"
```

---

### Task 2: Fix .env.example — complete all required vars

**Files:**
- Modify: `.env.example`

- [ ] **Step 1: Replace the entire file content**

```dotenv
## ── Clara Agents — Environment Variables ──────────────────────────────────────
## Copy to .env and fill in the REQUIRED values.
## Generate secrets:  openssl rand -hex 32
## Never commit .env to git.

# ── App ────────────────────────────────────────────────────────────────────────
ENVIRONMENT=development
FRONTEND_PORT=3000
BACKEND_PORT=3001

# ── Database — MySQL (REQUIRED) ────────────────────────────────────────────────
# Format: mysql://user:password@host:port/dbname?parseTime=true
# When running with docker-compose the compose file sets this automatically via
# the MYSQL_* vars below. For a standalone backend, set it here directly:
DATABASE_URL=mysql://clara:clarapassword@localhost:3306/claraverse?parseTime=true

# MySQL service credentials (used by docker-compose mysql service)
MYSQL_DATABASE=claraverse
MYSQL_USER=clara
MYSQL_PASSWORD=clarapassword
MYSQL_ROOT_PASSWORD=rootpassword
MYSQL_PORT=3306

# ── MongoDB (compose sets this automatically; override for external instance) ──
# MONGODB_URI=mongodb://localhost:27017/clara-agents

# ── Redis (compose sets this automatically; override for external instance) ────
# REDIS_URL=redis://localhost:6379

# ── URLs ───────────────────────────────────────────────────────────────────────
VITE_API_BASE_URL=http://localhost:3001
VITE_WS_URL=ws://localhost:3001
FRONTEND_URL=http://localhost:3000
BACKEND_URL=http://localhost:3001
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173

# ── Auth (REQUIRED — generate: openssl rand -hex 32) ──────────────────────────
JWT_SECRET=
JWT_ACCESS_TOKEN_EXPIRY=15m
JWT_REFRESH_TOKEN_EXPIRY=168h

# ── Encryption (REQUIRED for credential vault — generate: openssl rand -hex 32) ─
ENCRYPTION_MASTER_KEY=

# ── Admin bootstrap (optional) ────────────────────────────────────────────────
# The FIRST registered user automatically gets admin role — no manual DB update
# needed. Use this to pre-authorize additional admins by their MongoDB user IDs.
# SUPERADMIN_USER_IDS=

# ── Optional integrations ──────────────────────────────────────────────────────
# Web search tool block (run SearXNG: docker run -d -p 8080:8080 searxng/searxng)
# SEARXNG_URL=http://host.docker.internal:8080

# Sandboxed code execution (https://e2b.dev)
# E2B_API_KEY=

# Composio integrations (Google Sheets, Gmail, Calendar, etc.)
# COMPOSIO_API_KEY=
```

- [ ] **Step 2: Commit**

```bash
git add .env.example
git commit -m "fix: complete .env.example with DATABASE_URL, MySQL credentials, and all required vars"
```

---

### Task 3: ExecutionHandler — GET /api/executions, /api/executions/:id, /api/agents/:id/executions

**Files:**
- Create: `backend/internal/handlers/execution.go`

- [ ] **Step 1: Create execution.go**

```go
package handlers

import (
	"clara-agents/internal/services"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ExecutionHandler handles execution history endpoints.
type ExecutionHandler struct {
	executionService *services.ExecutionService
}

// NewExecutionHandler creates a new ExecutionHandler.
func NewExecutionHandler(executionService *services.ExecutionService) *ExecutionHandler {
	return &ExecutionHandler{executionService: executionService}
}

// ListByAgent returns paginated executions for a specific agent.
// GET /api/agents/:id/executions
func (h *ExecutionHandler) ListByAgent(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	agentID := c.Params("id")
	opts := &services.ListExecutionsOptions{
		Limit:  parseIntQuery(c, "limit", 20),
		Offset: parseIntQuery(c, "offset", 0),
		Status: c.Query("status"),
	}
	result, err := h.executionService.ListByAgent(c.Context(), agentID, userID, opts)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// ListByUser returns all executions for the authenticated user across agents.
// GET /api/executions
func (h *ExecutionHandler) ListByUser(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	opts := &services.ListExecutionsOptions{
		Limit:  parseIntQuery(c, "limit", 20),
		Offset: parseIntQuery(c, "offset", 0),
		Status: c.Query("status"),
	}
	result, err := h.executionService.ListByUser(c.Context(), userID, opts)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// GetByID returns a single execution record by ID.
// GET /api/executions/:id
func (h *ExecutionHandler) GetByID(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	executionID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid execution ID"})
	}
	record, err := h.executionService.GetByIDAndUser(c.Context(), executionID, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "execution not found"})
	}
	return c.JSON(record)
}

// parseIntQuery reads a query param as int with a fallback default.
func parseIntQuery(c *fiber.Ctx, key string, defaultVal int) int {
	v := c.QueryInt(key, defaultVal)
	if v <= 0 {
		return defaultVal
	}
	return v
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./internal/handlers/`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handlers/execution.go
git commit -m "feat: add ExecutionHandler for GET /api/executions and /api/agents/:id/executions"
```

---

### Task 4: Schedule usage route — GET /api/schedules/usage

`SchedulerService.GetScheduleUsage` already exists. This task wires it as an HTTP route.

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Add schedule usage route**

In `main.go`, find:
```go
	triggerGroup.Get("/:agentId/status/:executionId", triggerHandler.GetExecutionStatus)
```

Add this block immediately after:
```go
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
```

- [ ] **Step 2: Build to verify**

Run: `cd backend && go build ./cmd/server/`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat: add GET /api/schedules/usage route"
```

---

### Task 5: BulkUpdateTier — service method + handler + route

**Files:**
- Modify: `backend/internal/services/model_management_service.go`
- Modify: `backend/internal/handlers/model_management.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Add BulkUpdateTier to ModelManagementService**

In `backend/internal/services/model_management_service.go`, after the `BulkUpdateVisibility` method, add:

```go
// BulkUpdateTier sets the recommendation tier for a set of models.
// Pass an empty string to clear the tier (sets recommendation_tier to NULL).
func (s *ModelManagementService) BulkUpdateTier(modelIDs []string, tier string) error {
	if len(modelIDs) == 0 {
		return fmt.Errorf("no model IDs provided")
	}
	validTiers := map[string]bool{"top": true, "medium": true, "fastest": true, "new": true, "": true}
	if !validTiers[tier] {
		return fmt.Errorf("invalid tier: %s", tier)
	}

	placeholders := make([]string, len(modelIDs))
	args := make([]interface{}, len(modelIDs)+1)
	args[0] = tier
	for i, modelID := range modelIDs {
		placeholders[i] = "?"
		args[i+1] = modelID
	}

	query := fmt.Sprintf(`
		UPDATE models
		SET recommendation_tier = NULLIF(?, '')
		WHERE id IN (%s)
	`, strings.Join(placeholders, ","))

	result, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to bulk update tier: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	log.Printf("✅ [BULK] Updated recommendation_tier=%q for %d models", tier, rowsAffected)
	return nil
}
```

- [ ] **Step 2: Add BulkUpdateTier handler method**

In `backend/internal/handlers/model_management.go`, after `BulkUpdateVisibility`, add:

```go
// BulkUpdateTier sets the recommendation tier for a set of models.
// PUT /api/admin/models/bulk/tier
func (h *ModelManagementHandler) BulkUpdateTier(c *fiber.Ctx) error {
	adminUserID := c.Locals("user_id").(string)

	var req struct {
		ModelIDs []string `json:"model_ids"`
		Tier     string   `json:"tier"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if len(req.ModelIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "model_ids array is required"})
	}

	log.Printf("📝 Admin %s bulk updating tier=%q for %d models", adminUserID, req.Tier, len(req.ModelIDs))

	if err := h.modelMgmtService.BulkUpdateTier(req.ModelIDs, req.Tier); err != nil {
		log.Printf("❌ Failed to bulk update tier: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to update models: %v", err),
		})
	}
	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Updated tier=%q for %d models", req.Tier, len(req.ModelIDs)),
	})
}
```

- [ ] **Step 3: Register the route in main.go**

Find in `backend/cmd/server/main.go`:
```go
	adminGroup.Put("/models/bulk/visibility", modelMgmtHnd.BulkUpdateVisibility)
```

Add after it:
```go
	adminGroup.Put("/models/bulk/tier", modelMgmtHnd.BulkUpdateTier)
```

- [ ] **Step 4: Build to verify**

Run: `cd backend && go build ./...`
Expected: No errors.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/model_management_service.go \
        backend/internal/handlers/model_management.go \
        backend/cmd/server/main.go
git commit -m "feat: add PUT /api/admin/models/bulk/tier route"
```

---

### Task 6: Tools registry route — GET /api/tools/registry

**Files:**
- Modify: `backend/internal/handlers/tools.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Add ListRegistry to ToolsHandler**

In `backend/internal/handlers/tools.go`, after the closing brace of `ListTools`, add:

```go
// ListRegistry is an alias for ListTools served at /api/tools/registry.
// GET /api/tools/registry
func (h *ToolsHandler) ListRegistry(c *fiber.Ctx) error {
	return h.ListTools(c)
}
```

- [ ] **Step 2: Register the route in main.go**

Find:
```go
	apiGroup.Get("/tools", toolsHnd.ListTools)
```

Add after it:
```go
	apiGroup.Get("/tools/registry", toolsHnd.ListRegistry)
```

- [ ] **Step 3: Build to verify**

Run: `cd backend && go build ./...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handlers/tools.go backend/cmd/server/main.go
git commit -m "feat: add GET /api/tools/registry route"
```

---

### Task 7: Add GetModelAnalytics and GetCollectionStats to AnalyticsService

**Files:**
- Modify: `backend/internal/services/analytics_service.go`

- [ ] **Step 1: Verify field names on the Model struct returned by ModelService.GetAll**

Run: `cd backend && grep -n "ModelID\|ProviderName\|AgentsEnabled\|RecommendationTier\|func.*GetAll" internal/services/model_service.go | head -20`

If the field names differ from `ModelID`, `Name`, `ProviderName`, `AgentsEnabled`, `RecommendationTier`, note the actual names — you will use them in Step 2.

- [ ] **Step 2: Add GetModelAnalytics after GetProviderAnalytics**

In `backend/internal/services/analytics_service.go`, after `GetProviderAnalytics`, add:

```go
// GetModelAnalytics returns per-model usage counts joined with MySQL model data.
func (s *AnalyticsService) GetModelAnalytics(ctx context.Context, modelService *ModelService) ([]map[string]interface{}, error) {
	models, err := modelService.GetAll(false)
	if err != nil || len(models) == 0 {
		return []map[string]interface{}{}, nil
	}

	usageMap := make(map[string]int64)
	if s.mongoDB != nil {
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{"modelId": bson.M{"$exists": true, "$ne": ""}}}},
			{{Key: "$group", Value: bson.M{
				"_id":         "$modelId",
				"usage_count": bson.M{"$sum": 1},
			}}},
		}
		cursor, err := s.collection("chat_sessions").Aggregate(ctx, pipeline)
		if err == nil {
			var results []bson.M
			if err := cursor.All(ctx, &results); err == nil {
				for _, r := range results {
					if id, ok := r["_id"].(string); ok {
						usageMap[id] = extractInt64(r, "usage_count")
					}
				}
			}
		}
	}

	analytics := make([]map[string]interface{}, 0, len(models))
	for _, m := range models {
		var tier interface{}
		if m.RecommendationTier != "" {
			tier = m.RecommendationTier
		}
		analytics = append(analytics, map[string]interface{}{
			"model_id":            m.ModelID,
			"model_name":          m.Name,
			"provider_name":       m.ProviderName,
			"usage_count":         usageMap[m.ModelID],
			"agents_enabled":      m.AgentsEnabled,
			"recommendation_tier": tier,
		})
	}
	return analytics, nil
}
```

> If Step 1 revealed different field names, substitute them here.

- [ ] **Step 3: Add GetCollectionStats after GetModelAnalytics**

```go
// GetCollectionStats returns raw document counts from all major MongoDB collections.
func (s *AnalyticsService) GetCollectionStats(ctx context.Context) (map[string]interface{}, error) {
	empty := map[string]interface{}{
		"totalUsers": 0, "totalChats": 0, "totalAgents": 0,
		"totalExecutions": 0, "totalWorkflows": 0, "totalFeedback": 0,
		"totalTelemetryEvents": 0, "totalSubscriptions": 0,
		"userSignupsByDay":  []interface{}{},
		"chatsByDay":        []interface{}{},
		"executionsByDay":   []interface{}{},
		"topAgents":         []interface{}{},
	}
	if s.mongoDB == nil {
		return empty, nil
	}

	db := s.mongoDB.Database()
	count := func(coll string) int64 {
		n, _ := db.Collection(coll).CountDocuments(ctx, bson.M{})
		return n
	}

	dailySeries := func(coll, dateField string, days int) []map[string]interface{} {
		start := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Truncate(24 * time.Hour)
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{dateField: bson.M{"$gte": start}}}},
			{{Key: "$group", Value: bson.M{
				"_id":   bson.M{"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$" + dateField}},
				"value": bson.M{"$sum": 1},
			}}},
			{{Key: "$sort", Value: bson.M{"_id": 1}}},
		}
		cursor, err := db.Collection(coll).Aggregate(ctx, pipeline)
		if err != nil {
			return nil
		}
		var raw []bson.M
		cursor.All(ctx, &raw)
		result := make([]map[string]interface{}, 0, len(raw))
		for _, r := range raw {
			result = append(result, map[string]interface{}{"date": r["_id"], "value": extractInt64(r, "value")})
		}
		return result
	}

	topPipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{"_id": "$agentId", "count": bson.M{"$sum": 1}}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
		{{Key: "$limit", Value: 5}},
	}
	topCursor, _ := db.Collection("executions").Aggregate(ctx, topPipeline)
	topAgents := []map[string]interface{}{}
	if topCursor != nil {
		var topRaw []bson.M
		topCursor.All(ctx, &topRaw)
		for _, r := range topRaw {
			topAgents = append(topAgents, map[string]interface{}{"name": r["_id"], "count": extractInt64(r, "count")})
		}
	}

	return map[string]interface{}{
		"totalUsers":           count("users"),
		"totalChats":           count("conversations"),
		"totalAgents":          count("agents"),
		"totalExecutions":      count("executions"),
		"totalWorkflows":       count("workflows"),
		"totalFeedback":        count("feedback"),
		"totalTelemetryEvents": count("telemetry_events"),
		"totalSubscriptions":   count("subscriptions"),
		"userSignupsByDay":     dailySeries("users", "createdAt", 30),
		"chatsByDay":           dailySeries("conversations", "createdAt", 30),
		"executionsByDay":      dailySeries("executions", "startedAt", 30),
		"topAgents":            topAgents,
	}, nil
}
```

- [ ] **Step 4: Build to verify**

Run: `cd backend && go build ./internal/services/`
Expected: No errors. If `m.RecommendationTier` or other fields don't exist, fix the field names based on Step 1 output.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/analytics_service.go
git commit -m "feat: add GetModelAnalytics and GetCollectionStats to AnalyticsService"
```

---

### Task 8: Create AdminAnalyticsHandler

**Files:**
- Create: `backend/internal/handlers/analytics_admin.go`

- [ ] **Step 1: Create analytics_admin.go**

```go
package handlers

import (
	"clara-agents/internal/health"
	"clara-agents/internal/services"
	"time"

	"github.com/gofiber/fiber/v2"
)

// AdminAnalyticsHandler serves all admin analytics, insights, health, user, and autopilot endpoints.
type AdminAnalyticsHandler struct {
	analyticsService *services.AnalyticsService
	modelService     *services.ModelService
	healthService    *health.Service
}

// NewAdminAnalyticsHandler creates an AdminAnalyticsHandler.
func NewAdminAnalyticsHandler(
	analyticsService *services.AnalyticsService,
	modelService *services.ModelService,
	healthService *health.Service,
) *AdminAnalyticsHandler {
	return &AdminAnalyticsHandler{
		analyticsService: analyticsService,
		modelService:     modelService,
		healthService:    healthService,
	}
}

// ── Analytics ─────────────────────────────────────────────────────────────────

// GetOverviewStats — GET /api/admin/analytics/overview
func (h *AdminAnalyticsHandler) GetOverviewStats(c *fiber.Ctx) error {
	stats, err := h.analyticsService.GetOverviewStats(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(stats)
}

// GetProviderAnalytics — GET /api/admin/analytics/providers
func (h *AdminAnalyticsHandler) GetProviderAnalytics(c *fiber.Ctx) error {
	data, err := h.analyticsService.GetProviderAnalytics(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(data)
}

// GetModelAnalytics — GET /api/admin/analytics/models
func (h *AdminAnalyticsHandler) GetModelAnalytics(c *fiber.Ctx) error {
	data, err := h.analyticsService.GetModelAnalytics(c.Context(), h.modelService)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(data)
}

// GetChatAnalytics — GET /api/admin/analytics/chats
func (h *AdminAnalyticsHandler) GetChatAnalytics(c *fiber.Ctx) error {
	data, err := h.analyticsService.GetChatAnalytics(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(data)
}

// GetAgentAnalytics — GET /api/admin/analytics/agents
func (h *AdminAnalyticsHandler) GetAgentAnalytics(c *fiber.Ctx) error {
	data, err := h.analyticsService.GetAgentAnalytics(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(data)
}

// MigrateChatTimestamps — POST /api/admin/analytics/migrate-timestamps
func (h *AdminAnalyticsHandler) MigrateChatTimestamps(c *fiber.Ctx) error {
	n, err := h.analyticsService.MigrateChatSessionTimestamps(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true, "message": "Migration complete", "sessions_updated": n})
}

// ── Users + GDPR ──────────────────────────────────────────────────────────────

// GetUsers — GET /api/admin/users
func (h *AdminAnalyticsHandler) GetUsers(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("page_size", 20)
	tier := c.Query("tier")
	search := c.Query("search")

	users, total, err := h.analyticsService.GetUserListGDPR(c.Context(), page, pageSize, tier, search)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"users":       users,
		"total_count": total,
		"page":        page,
		"page_size":   pageSize,
		"gdpr_notice": "User data is anonymized. Only aggregate analytics and email domains are shown.",
	})
}

// GetGDPRPolicy — GET /api/admin/gdpr-policy
func (h *AdminAnalyticsHandler) GetGDPRPolicy(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"data_collected": []string{
			"Email address (hashed for analytics)",
			"Chat session metadata (start time, message count, model used)",
			"Agent execution logs (agent ID, trigger type, timestamp)",
			"Authentication tokens (stored as hashed values)",
		},
		"data_retention_days": 365,
		"purpose":             "Service operation, abuse prevention, and aggregate usage analytics",
		"legal_basis":         "Legitimate interest (service operation) and user consent for analytics",
		"user_rights":         []string{"Access", "Rectification", "Erasure", "Data portability", "Restrict processing"},
	})
}

// ── Health Dashboard ──────────────────────────────────────────────────────────

// GetHealthDashboard — GET /api/admin/health
func (h *AdminAnalyticsHandler) GetHealthDashboard(c *fiber.Ctx) error {
	if h.healthService == nil {
		return c.JSON(fiber.Map{
			"summary":      fiber.Map{"total": 0, "healthy": 0, "unhealthy": 0, "cooldown": 0, "unknown": 0},
			"capabilities": fiber.Map{},
			"providers":    []interface{}{},
		})
	}

	all := h.healthService.GetAllRegistered()
	summary := struct {
		Total     int `json:"total"`
		Healthy   int `json:"healthy"`
		Unhealthy int `json:"unhealthy"`
		Cooldown  int `json:"cooldown"`
		Unknown   int `json:"unknown"`
	}{}
	capMap := make(map[string]map[string]int)
	providerEntries := make([]fiber.Map, 0, len(all))

	for _, p := range all {
		summary.Total++
		cap := string(p.Capability)
		if _, ok := capMap[cap]; !ok {
			capMap[cap] = map[string]int{"healthy": 0, "unhealthy": 0, "cooldown": 0, "unknown": 0}
		}

		var statusStr string
		switch p.Status {
		case health.StatusHealthy:
			statusStr = "healthy"
			summary.Healthy++
			capMap[cap]["healthy"]++
		case health.StatusUnhealthy:
			statusStr = "unhealthy"
			summary.Unhealthy++
			capMap[cap]["unhealthy"]++
		case health.StatusCooldown:
			statusStr = "cooldown"
			summary.Cooldown++
			capMap[cap]["cooldown"]++
		default:
			statusStr = "unknown"
			summary.Unknown++
			capMap[cap]["unknown"]++
		}

		var lastChecked, lastSuccess, cooldownUntil interface{}
		if !p.LastChecked.IsZero() {
			lastChecked = p.LastChecked.Format(time.RFC3339)
		}
		if !p.LastSuccessAt.IsZero() {
			lastSuccess = p.LastSuccessAt.Format(time.RFC3339)
		}
		if !p.CooldownUntil.IsZero() {
			cooldownUntil = p.CooldownUntil.Format(time.RFC3339)
		}

		providerEntries = append(providerEntries, fiber.Map{
			"provider_id":    p.ProviderID,
			"provider_name":  p.ProviderName,
			"model_name":     p.ModelName,
			"capability":     cap,
			"status":         statusStr,
			"failure_count":  p.FailureCount,
			"last_error":     p.LastError,
			"last_checked":   lastChecked,
			"last_success":   lastSuccess,
			"cooldown_until": cooldownUntil,
			"priority":       p.Priority,
		})
	}

	return c.JSON(fiber.Map{
		"summary":      summary,
		"capabilities": capMap,
		"providers":    providerEntries,
	})
}

// ── Insights (stubs — return well-shaped empty data so frontend renders) ──────

// GetInsightsOverview — GET /api/admin/insights/overview
func (h *AdminAnalyticsHandler) GetInsightsOverview(c *fiber.Ctx) error {
	now := time.Now()
	return c.JSON(fiber.Map{
		"period":   fiber.Map{"start": now.Add(-30 * 24 * time.Hour).Format("2006-01-02"), "end": now.Format("2006-01-02")},
		"users":    fiber.Map{"totalActive": 0, "trend": "0%", "new7d": 0, "churned7d": 0, "atRisk": 0},
		"features": fiber.Map{},
		"feedback": fiber.Map{"bugs7d": 0, "features7d": 0, "npsAvg": 0, "npsTrend": "0%"},
		"errors":   fiber.Map{"total7d": 0, "trend": "0%", "topType": ""},
	})
}

// GetInsightsMetrics — GET /api/admin/insights/metrics
func (h *AdminAnalyticsHandler) GetInsightsMetrics(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"metrics": []interface{}{}})
}

// GetHealthDistribution — GET /api/admin/insights/health-distribution
func (h *AdminAnalyticsHandler) GetHealthDistribution(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"healthy": 0, "medium": 0, "high": 0, "signals": fiber.Map{}})
}

// GetActivationFunnel — GET /api/admin/insights/activation-funnel
func (h *AdminAnalyticsHandler) GetActivationFunnel(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"funnel": []interface{}{}, "biggestDropOff": "", "dropOffRate": 0})
}

// GetFeedbackStream — GET /api/admin/insights/feedback-stream
func (h *AdminAnalyticsHandler) GetFeedbackStream(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"feedback": []interface{}{}, "totalCount": 0})
}

// GetCollectionStats — GET /api/admin/insights/collection-stats
func (h *AdminAnalyticsHandler) GetCollectionStats(c *fiber.Ctx) error {
	stats, err := h.analyticsService.GetCollectionStats(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(stats)
}

// BackfillMetrics — POST /api/admin/insights/backfill
func (h *AdminAnalyticsHandler) BackfillMetrics(c *fiber.Ctx) error {
	days := c.QueryInt("days", 90)
	return c.JSON(fiber.Map{"message": "Backfill not yet implemented", "days": days, "processed": 0})
}

// ── AutoPilot (stubs) ─────────────────────────────────────────────────────────

// GetAutoPilotContext — GET /api/admin/autopilot/context
func (h *AdminAnalyticsHandler) GetAutoPilotContext(c *fiber.Ctx) error {
	now := time.Now()
	return c.JSON(fiber.Map{
		"generatedAt": now.Format(time.RFC3339),
		"period":      "30d",
		"context": fiber.Map{
			"users":    fiber.Map{"totalActive": 0, "trend": "0%", "new7d": 0, "churned7d": 0, "atRisk": 0},
			"features": fiber.Map{},
			"feedback": fiber.Map{"bugs7d": 0, "features7d": 0, "npsAvg": 0, "npsTrend": "0%"},
			"errors":   fiber.Map{"total7d": 0, "trend": "0%", "topType": ""},
			"health":   fiber.Map{"healthy": 0, "medium": 0, "high": 0, "signals": fiber.Map{}},
			"funnel":   fiber.Map{"funnel": []interface{}{}, "biggestDropOff": "", "dropOffRate": 0},
			"trends":   fiber.Map{},
		},
		"potentialActions": []interface{}{},
	})
}

// AnalyzeWithAI — POST /api/admin/autopilot/analyze
func (h *AdminAnalyticsHandler) AnalyzeWithAI(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"analysis":  "AutoPilot AI analysis is not yet configured. Add a model provider in /admin to enable AI-powered insights.",
		"model":     "",
		"tokens":    fiber.Map{"input": 0, "output": 0},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
```

- [ ] **Step 2: Build to verify**

Run: `cd backend && go build ./internal/handlers/`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handlers/analytics_admin.go
git commit -m "feat: add AdminAnalyticsHandler for all admin analytics, health, insights, autopilot endpoints"
```

---

### Task 9: Wire all new handlers in main.go

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Initialize AnalyticsService — add after the `if mongoDB != nil` block closes**

Find the closing `}` of the `if mongoDB != nil` block (after line ~114 in main.go, following `webhookService = services.NewWebhookService(mongoDB)`). Add immediately after:

```go
	// AnalyticsService handles nil MongoDB internally — always safe to create
	analyticsService := services.NewAnalyticsService(mongoDB)
	if mongoDB != nil {
		_ = analyticsService.EnsureIndexes(context.Background())
	}
```

- [ ] **Step 2: Register execution routes — add after the agentGroup webhook routes**

Find:
```go
	agentGroup.Delete("/:id/webhook", agentHandler.DeleteWebhook)
```

Add after it:
```go
	// Execution history
	if executionService != nil {
		execHnd := handlers.NewExecutionHandler(executionService)
		agentGroup.Get("/:id/executions", execHnd.ListByAgent)
		apiGroup.Get("/executions", execHnd.ListByUser)
		apiGroup.Get("/executions/:id", execHnd.GetByID)
	}
```

- [ ] **Step 3: Register admin analytics routes — add after the /tiers route**

Find:
```go
	adminGroup.Get("/tiers", modelMgmtHnd.GetTiers)
```

Add after it:
```go
	// Admin analytics, health, insights, users, autopilot
	analyticsHnd := handlers.NewAdminAnalyticsHandler(analyticsService, modelService, services.GetHealthService())
	adminGroup.Get("/analytics/overview", analyticsHnd.GetOverviewStats)
	adminGroup.Get("/analytics/providers", analyticsHnd.GetProviderAnalytics)
	adminGroup.Get("/analytics/models", analyticsHnd.GetModelAnalytics)
	adminGroup.Get("/analytics/chats", analyticsHnd.GetChatAnalytics)
	adminGroup.Get("/analytics/agents", analyticsHnd.GetAgentAnalytics)
	adminGroup.Post("/analytics/migrate-timestamps", analyticsHnd.MigrateChatTimestamps)
	adminGroup.Get("/users", analyticsHnd.GetUsers)
	adminGroup.Get("/gdpr-policy", analyticsHnd.GetGDPRPolicy)
	adminGroup.Get("/health", analyticsHnd.GetHealthDashboard)
	adminGroup.Get("/insights/overview", analyticsHnd.GetInsightsOverview)
	adminGroup.Get("/insights/metrics", analyticsHnd.GetInsightsMetrics)
	adminGroup.Get("/insights/health-distribution", analyticsHnd.GetHealthDistribution)
	adminGroup.Get("/insights/activation-funnel", analyticsHnd.GetActivationFunnel)
	adminGroup.Get("/insights/feedback-stream", analyticsHnd.GetFeedbackStream)
	adminGroup.Get("/insights/collection-stats", analyticsHnd.GetCollectionStats)
	adminGroup.Post("/insights/backfill", analyticsHnd.BackfillMetrics)
	adminGroup.Get("/autopilot/context", analyticsHnd.GetAutoPilotContext)
	adminGroup.Post("/autopilot/analyze", analyticsHnd.AnalyzeWithAI)
```

- [ ] **Step 4: Full build**

Run: `cd backend && go build ./...`
Expected: No errors. If `services.GetHealthService()` causes an import cycle, replace with `nil` — the handler already handles nil gracefully.

- [ ] **Step 5: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat: wire ExecutionHandler, AdminAnalyticsHandler, and all missing routes in main.go"
```

---

### Task 10: End-to-end smoke test

- [ ] **Step 1: Create .env and generate secrets**

```bash
cp .env.example .env
# Fill in the REQUIRED values:
echo "JWT_SECRET=$(openssl rand -hex 32)"
echo "ENCRYPTION_MASTER_KEY=$(openssl rand -hex 32)"
# Paste the output values into .env
```

- [ ] **Step 2: Start the full stack**

```bash
docker compose up --build -d
```

Wait ~60s, then check all services are healthy:

```bash
docker compose ps
```

Expected: All 5 services (`frontend`, `backend`, `mysql`, `mongodb`, `redis`) show `healthy`.

- [ ] **Step 3: Verify backend started cleanly**

```bash
docker compose logs backend | grep -E "✅|❌|🌐 Listening"
```

Expected:
```
✅ MongoDB connected
✅ Redis connected
✅ JWT auth ready
🌐 Listening on :3001
```

Must NOT contain: `❌ DATABASE_URL is required`

- [ ] **Step 4: Register the first user — should automatically get admin**

```bash
curl -s -X POST http://localhost:3001/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"Admin1234!"}' | jq .role
```

Expected output: `"admin"`

- [ ] **Step 5: Test previously-missing routes now return 200**

```bash
TOKEN=$(curl -s -X POST http://localhost:3001/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"Admin1234!"}' | jq -r .access_token)

curl -s -H "Authorization: Bearer $TOKEN" http://localhost:3001/api/executions | jq .has_more
# Expected: false

curl -s -H "Authorization: Bearer $TOKEN" http://localhost:3001/api/schedules/usage | jq .limit
# Expected: 5

curl -s -H "Authorization: Bearer $TOKEN" http://localhost:3001/api/admin/analytics/overview | jq .total_users
# Expected: 1

curl -s -H "Authorization: Bearer $TOKEN" http://localhost:3001/api/admin/health | jq .summary.total
# Expected: a number (0 if no providers configured yet)

curl -s -H "Authorization: Bearer $TOKEN" http://localhost:3001/api/admin/gdpr-policy | jq .legal_basis
# Expected: a string

curl -s -H "Authorization: Bearer $TOKEN" http://localhost:3001/api/admin/insights/collection-stats | jq .totalUsers
# Expected: 1
```

- [ ] **Step 6: Open the app in browser and verify /admin loads**

Visit `http://localhost:3000`.
- Register/login → should work  
- Navigate to `/admin` → admin panel loads without errors
- Navigate to `/admin` analytics tabs → should show data (or zeros) without 404 errors

- [ ] **Step 7: Commit any last-minute fixes**

```bash
git add -p
git commit -m "fix: smoke test fixes"
```
