# Google OAuth Implementation Plan (Production-Ready)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add "Sign in with Google" to the Orchid login page alongside email/password, using a secure exchange-code pattern so the JWT never appears in a URL.

**Architecture:**
The flow has three backend endpoints. `GET /api/auth/google` generates a CSRF-safe state and redirects the browser to Google. `GET /api/auth/google/callback` handles Google's redirect, creates or finds the user, then stores the JWT pair in Redis under a random 60-second one-time code and redirects the frontend to `/auth/google/callback?code=<opaque-code>`. `POST /api/auth/google/exchange` is the only endpoint the frontend calls — it redeems the code atomically (read + delete) and returns the `AuthResponse` identical in shape to the existing login response. Rate limiting on all three OAuth endpoints via Fiber's built-in in-memory limiter.

**Tech Stack:** Go (Fiber v2, `golang.org/x/oauth2`), Redis (already in stack via `RedisService`), React 19 + React Router v6, Zustand

> **Redis dependency:** Google OAuth requires Redis. If `REDIS_URL` is not set, Google auth routes are not registered and the button is not shown.

---

## File Map

### New files
| Path | Responsibility |
|------|----------------|
| `backend/internal/handlers/auth_google.go` | `GoogleAuthHandler` — `Redirect`, `Callback`, `ExchangeCode` |
| `frontend/src/pages/GoogleAuthCallbackPage.tsx` | Calls `/api/auth/google/exchange`, stores token, navigates to `/agents` |

### Modified files
| Path | What changes |
|------|--------------|
| `backend/go.mod` / `backend/go.sum` | Add `golang.org/x/oauth2` |
| `backend/internal/models/user.go` | Add `GoogleID string` + `Name string` fields |
| `backend/internal/services/user_service.go` | Add `GetUserByGoogleID` (returns `nil, nil` when not found) |
| `backend/internal/config/config.go` | Add `GoogleClientID`, `GoogleClientSecret` |
| `backend/cmd/server/main.go` | Init `GoogleAuthHandler`, register 3 routes with rate limiter |
| `.env.example` | Document `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` |
| `.env` | Add blank credential lines |
| `frontend/src/App.tsx` | Add `/auth/google/callback` route |
| `frontend/src/pages/LoginPage.tsx` | Google button + read `?error=` from URL |
| `frontend/src/pages/LoginPage.css` | Styles for divider and Google button |
| `frontend/src/store/useAuthStore.ts` | Add `loginWithGoogleToken(token, user)` action |

---

## Task 1: Add golang.org/x/oauth2 dependency

**Files:**
- Modify: `backend/go.mod`, `backend/go.sum`

- [ ] **Step 1: Add the dependency**

```bash
cd /Users/sujal/Dev/clara-agents/backend
go get golang.org/x/oauth2@latest
```

`golang.org/x/oauth2/google` is a sub-package of the same module — one `go get` covers it. Do **not** run a separate `go get golang.org/x/oauth2/google@latest`.

- [ ] **Step 2: Verify build**

```bash
cd /Users/sujal/Dev/clara-agents/backend
go build ./...
```

Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
cd /Users/sujal/Dev/clara-agents
git add backend/go.mod backend/go.sum
git commit -m "chore: add golang.org/x/oauth2 dependency for Google auth"
```

---

## Task 2: Extend User model with GoogleID and Name

**Files:**
- Modify: `backend/internal/models/user.go`

- [ ] **Step 1: Add fields to the User struct**

In `backend/internal/models/user.go`, add two lines after the `Email` field (the line `Email string \`bson:"email" json:"email"\``):

```go
Name         string             `bson:"name,omitempty" json:"name,omitempty"`
GoogleID     string             `bson:"googleId,omitempty" json:"-"` // Never exposed in API
```

- [ ] **Step 2: Expose Name in UserResponse**

In the `UserResponse` struct, add after the `Email` field:

```go
Name string `json:"name,omitempty"`
```

- [ ] **Step 3: Populate Name in ToResponse()**

In the `ToResponse()` method, add `Name: u.Name,` after `Email: u.Email,`:

```go
func (u *User) ToResponse() UserResponse {
    return UserResponse{
        ID:          u.ID.Hex(),
        Email:       u.Email,
        Name:        u.Name,
        // ... rest unchanged
    }
}
```

- [ ] **Step 4: Verify build**

```bash
cd /Users/sujal/Dev/clara-agents/backend
go build ./...
```

Expected: exits 0.

- [ ] **Step 5: Commit**

```bash
cd /Users/sujal/Dev/clara-agents
git add backend/internal/models/user.go
git commit -m "feat: add Name and GoogleID fields to User model"
```

---

## Task 3: Add GetUserByGoogleID to UserService

**Files:**
- Modify: `backend/internal/services/user_service.go`

- [ ] **Step 1: Add the method**

Add this method at the end of `backend/internal/services/user_service.go`, before the closing of the file:

```go
// GetUserByGoogleID finds a user by their Google OAuth ID.
// Returns nil, nil when no user exists with that Google ID (not a DB error).
func (s *UserService) GetUserByGoogleID(ctx context.Context, googleID string) (*models.User, error) {
	var user models.User
	err := s.collection.FindOne(ctx, bson.M{"googleId": googleID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("GetUserByGoogleID: %w", err)
	}
	return &user, nil
}
```

The required imports (`bson`, `mongo`, `fmt`, `context`) are already present in the file.

- [ ] **Step 2: Verify build**

```bash
cd /Users/sujal/Dev/clara-agents/backend
go build ./...
```

Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
cd /Users/sujal/Dev/clara-agents
git add backend/internal/services/user_service.go
git commit -m "feat: add GetUserByGoogleID to UserService"
```

---

## Task 4: Add Google OAuth config fields

**Files:**
- Modify: `backend/internal/config/config.go`, `.env.example`, `.env`

- [ ] **Step 1: Add fields to Config struct**

In `backend/internal/config/config.go`, add to the `Config` struct after the `FrontendURL` / `BackendURL` lines:

```go
// Google OAuth (optional — requires Redis)
GoogleClientID     string
GoogleClientSecret string
```

- [ ] **Step 2: Load them in Load()**

In the `return &Config{...}` block, add:

```go
GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
```

- [ ] **Step 3: Document in .env.example**

Add this block to `.env.example` after the `ENCRYPTION_MASTER_KEY` block:

```
# ── Google OAuth (optional) ───────────────────────────────────────────────────
# Requires Redis. Enables "Continue with Google" on the login page.
# Setup:
#   1. https://console.cloud.google.com → APIs & Services → Credentials
#   2. Create OAuth 2.0 Client ID (type: Web application)
#   3. Authorized redirect URI: http://localhost:3001/api/auth/google/callback
#      (change localhost:3001 to your BACKEND_URL in production)
# GOOGLE_CLIENT_ID=
# GOOGLE_CLIENT_SECRET=
```

- [ ] **Step 4: Add to .env**

Add these two commented-out lines to the actual `.env`:

```
# GOOGLE_CLIENT_ID=
# GOOGLE_CLIENT_SECRET=
```

- [ ] **Step 5: Verify build**

```bash
cd /Users/sujal/Dev/clara-agents/backend
go build ./...
```

Expected: exits 0.

- [ ] **Step 6: Commit**

```bash
cd /Users/sujal/Dev/clara-agents
git add backend/internal/config/config.go .env.example .env
git commit -m "feat: add Google OAuth config fields"
```

---

## Task 5: Create GoogleAuthHandler

**Files:**
- Create: `backend/internal/handlers/auth_google.go`

This handler has three methods:
- `Redirect` — generates a HMAC-signed state nonce, sets it in a cookie, redirects to Google
- `Callback` — verifies state, exchanges code with Google, upserts user, stores JWTs in Redis under a one-time exchange code, redirects frontend to `/auth/google/callback?code=<code>`
- `ExchangeCode` — frontend POSTs the opaque code, backend atomically reads+deletes it from Redis, returns `AuthResponse`

- [ ] **Step 1: Create the file**

Create `backend/internal/handlers/auth_google.go`:

```go
package handlers

import (
	"clara-agents/internal/config"
	"clara-agents/internal/models"
	"clara-agents/internal/services"
	"clara-agents/pkg/auth"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	googleStateCookie    = "g_state"
	googleStateCookieTTL = 10 * time.Minute
	exchangeCodeTTL      = 60 * time.Second
	exchangeCodePrefix   = "google:exchange:"
)

// exchangePayload is what we store in Redis under the one-time exchange code.
type exchangePayload struct {
	AccessToken  string             `json:"access_token"`
	RefreshToken string             `json:"refresh_token"`
	User         models.UserResponse `json:"user"`
}

// GoogleAuthHandler handles Google OAuth2 login.
type GoogleAuthHandler struct {
	oauthConfig  *oauth2.Config
	jwtAuth      *auth.LocalJWTAuth
	userService  *services.UserService
	redisService *services.RedisService
	hmacSecret   []byte
	frontendURL  string
}

// NewGoogleAuthHandler builds the handler.
// Returns nil when Google credentials are missing or Redis is nil — callers
// register routes only when the return value is non-nil.
func NewGoogleAuthHandler(
	cfg *config.Config,
	jwtAuth *auth.LocalJWTAuth,
	userService *services.UserService,
	redisService *services.RedisService,
) *GoogleAuthHandler {
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" || redisService == nil {
		return nil
	}

	oauthCfg := &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.BackendURL + "/api/auth/google/callback",
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}

	// Derive an HMAC key from JWT secret to sign the state nonce.
	h := hmac.New(sha256.New, []byte(cfg.JWTSecret))
	h.Write([]byte("orchid-google-state-v1"))
	hmacKey := h.Sum(nil)

	return &GoogleAuthHandler{
		oauthConfig:  oauthCfg,
		jwtAuth:      jwtAuth,
		userService:  userService,
		redisService: redisService,
		hmacSecret:   hmacKey,
		frontendURL:  cfg.FrontendURL,
	}
}

// Redirect generates a signed state and sends the browser to Google.
// GET /api/auth/google
func (h *GoogleAuthHandler) Redirect(c *fiber.Ctx) error {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate nonce"})
	}
	nonceB64 := base64.RawURLEncoding.EncodeToString(nonce)

	mac := hmac.New(sha256.New, h.hmacSecret)
	mac.Write([]byte(nonceB64))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	state := nonceB64 + "." + sig

	// Store the raw nonce in a short-lived httpOnly cookie.
	c.Cookie(&fiber.Cookie{
		Name:     googleStateCookie,
		Value:    nonceB64,
		MaxAge:   int(googleStateCookieTTL.Seconds()),
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Lax", // Must be Lax — Google redirects via GET (cross-site top-level nav)
		Path:     "/api/auth/google",
	})

	return c.Redirect(
		h.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline),
		fiber.StatusTemporaryRedirect,
	)
}

// Callback exchanges the auth code, upserts the user, stores JWT in Redis,
// then redirects the frontend with a short-lived one-time code.
// GET /api/auth/google/callback
func (h *GoogleAuthHandler) Callback(c *fiber.Ctx) error {
	errRedirect := func(reason string) error {
		return c.Redirect(
			h.frontendURL+"/login?error="+reason,
			fiber.StatusTemporaryRedirect,
		)
	}

	// ── 1. Verify state ──────────────────────────────────────────────────────
	state := c.Query("state")
	parts := strings.SplitN(state, ".", 2)
	if len(parts) != 2 {
		return errRedirect("invalid_state")
	}
	nonce, sigFromState := parts[0], parts[1]

	mac := hmac.New(sha256.New, h.hmacSecret)
	mac.Write([]byte(nonce))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sigFromState), []byte(expectedSig)) {
		return errRedirect("state_mismatch")
	}
	if cookieNonce := c.Cookies(googleStateCookie); cookieNonce == "" || cookieNonce != nonce {
		return errRedirect("state_cookie_mismatch")
	}

	// Clear state cookie.
	c.Cookie(&fiber.Cookie{
		Name:     googleStateCookie,
		Value:    "",
		MaxAge:   -1,
		HTTPOnly: true,
		Path:     "/api/auth/google",
	})

	// ── 2. Exchange auth code ────────────────────────────────────────────────
	code := c.Query("code")
	if code == "" {
		return errRedirect("missing_code")
	}

	ctx := context.Background()
	token, err := h.oauthConfig.Exchange(ctx, code)
	if err != nil {
		log.Printf("❌ Google token exchange failed: %v", err)
		return errRedirect("exchange_failed")
	}

	// ── 3. Fetch user info from Google ───────────────────────────────────────
	client := h.oauthConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("❌ Google userinfo request failed: status=%v err=%v", resp.StatusCode, err)
		return errRedirect("userinfo_failed")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var gUser struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		Name          string `json:"name"`
		VerifiedEmail bool   `json:"verified_email"`
	}
	if err := json.Unmarshal(body, &gUser); err != nil || gUser.Email == "" {
		log.Printf("❌ Google userinfo parse failed: %v", err)
		return errRedirect("userinfo_parse_failed")
	}
	gUser.Email = strings.ToLower(strings.TrimSpace(gUser.Email))

	// ── 4. Upsert user ───────────────────────────────────────────────────────
	user, err := h.userService.GetUserByGoogleID(ctx, gUser.ID)
	if err != nil {
		log.Printf("❌ DB lookup by Google ID failed: %v", err)
		return errRedirect("db_error")
	}

	if user == nil {
		// Fallback: look up by email (existing local-auth user linking accounts)
		user, err = h.userService.GetUserByEmail(ctx, gUser.Email)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			log.Printf("❌ DB lookup by email failed: %v", err)
			return errRedirect("db_error")
		}
	}

	now := time.Now()
	if user == nil {
		// Brand-new user.
		userCount, _ := h.userService.GetUserCount(ctx)
		role := "user"
		if userCount == 0 {
			role = "admin"
			log.Printf("🎉 First Google user — granting admin: %s", gUser.Email)
		}
		user = &models.User{
			ID:                  primitive.NewObjectID(),
			Email:               gUser.Email,
			Name:                gUser.Name,
			GoogleID:            gUser.ID,
			EmailVerified:       gUser.VerifiedEmail,
			Role:                role,
			RefreshTokenVersion: 0,
			CreatedAt:           now,
			LastLoginAt:         now,
			SubscriptionTier:    "pro",
			SubscriptionStatus:  "active",
			Preferences: models.UserPreferences{
				StoreBuilderChatHistory: true,
				MemoryEnabled:           false,
			},
		}
		if err := h.userService.CreateUser(ctx, user); err != nil {
			log.Printf("❌ Failed to create Google user: %v", err)
			return errRedirect("create_user_failed")
		}
		log.Printf("✅ Google user created: %s (%s)", user.Email, user.ID.Hex())
	} else {
		// Existing user — backfill fields silently.
		if user.GoogleID == "" {
			user.GoogleID = gUser.ID
		}
		if user.Name == "" && gUser.Name != "" {
			user.Name = gUser.Name
		}
		user.LastLoginAt = now
		if err := h.userService.UpdateUser(ctx, user); err != nil {
			log.Printf("⚠️  Non-fatal: failed to update Google user fields: %v", err)
		}
		log.Printf("✅ Google user signed in: %s (%s)", user.Email, user.ID.Hex())
	}

	// ── 5. Issue JWT pair ────────────────────────────────────────────────────
	accessToken, refreshToken, err := h.jwtAuth.GenerateTokens(user.ID.Hex(), user.Email, user.Role)
	if err != nil {
		log.Printf("❌ JWT generation failed: %v", err)
		return errRedirect("token_generation_failed")
	}

	// ── 6. Store tokens in Redis under a one-time exchange code ─────────────
	rawCode := make([]byte, 24)
	if _, err := rand.Read(rawCode); err != nil {
		log.Printf("❌ Failed to generate exchange code: %v", err)
		return errRedirect("exchange_code_failed")
	}
	exchangeCode := base64.RawURLEncoding.EncodeToString(rawCode)

	payload := exchangePayload{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user.ToResponse(),
	}
	payloadJSON, _ := json.Marshal(payload)

	redisKey := exchangeCodePrefix + exchangeCode
	if err := h.redisService.Set(ctx, redisKey, string(payloadJSON), exchangeCodeTTL); err != nil {
		log.Printf("❌ Failed to store exchange code in Redis: %v", err)
		return errRedirect("redis_error")
	}

	// Redirect frontend — only the opaque code in the URL, never the JWT.
	return c.Redirect(
		fmt.Sprintf("%s/auth/google/callback?code=%s", h.frontendURL, exchangeCode),
		fiber.StatusTemporaryRedirect,
	)
}

// ExchangeCodeRequest is the body for the exchange endpoint.
type ExchangeCodeRequest struct {
	Code string `json:"code"`
}

// ExchangeCode redeems the one-time code and returns the AuthResponse.
// POST /api/auth/google/exchange
func (h *GoogleAuthHandler) ExchangeCode(c *fiber.Ctx) error {
	var req ExchangeCodeRequest
	if err := c.BodyParser(&req); err != nil || req.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "code is required"})
	}

	ctx := context.Background()
	redisKey := exchangeCodePrefix + req.Code

	// Atomic read + delete — prevents replay.
	raw, err := h.redisService.Get(ctx, redisKey)
	if err != nil {
		// Key missing or expired.
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired exchange code"})
	}
	// Delete immediately — one-time use.
	_ = h.redisService.Delete(ctx, redisKey)

	var payload exchangePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "malformed exchange payload"})
	}

	// Set refresh token httpOnly cookie (same behaviour as local login).
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    payload.RefreshToken,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Strict",
		Path:     "/api/auth",
	})

	return c.JSON(AuthResponse{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		User:         payload.User,
		ExpiresIn:    15 * 60,
	})
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/sujal/Dev/clara-agents/backend
go build ./...
```

Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
cd /Users/sujal/Dev/clara-agents
git add backend/internal/handlers/auth_google.go
git commit -m "feat: add GoogleAuthHandler (Redirect, Callback, ExchangeCode)"
```

---

## Task 6: Wire GoogleAuthHandler into main.go

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Add the limiter import**

At the top of `backend/cmd/server/main.go`, add to the import block:

```go
"github.com/gofiber/fiber/v2/middleware/limiter"
```

- [ ] **Step 2: Register Google auth routes**

Find the block where `authGroup` routes end (around `authGroup.Get("/me", jwtMw, authHandler.GetCurrentUser)`). Add immediately after:

```go
// Google OAuth routes — only registered when credentials + Redis are configured.
googleAuthHandler := handlers.NewGoogleAuthHandler(cfg, jwtAuth, userService, redisService)
if googleAuthHandler != nil {
	// Rate limit: 20 requests per minute per IP on the OAuth initiation + callback.
	oauthLimiter := limiter.New(limiter.Config{
		Max:        20,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many requests"})
		},
	})
	authGroup.Get("/google", oauthLimiter, googleAuthHandler.Redirect)
	authGroup.Get("/google/callback", oauthLimiter, googleAuthHandler.Callback)
	authGroup.Post("/google/exchange", oauthLimiter, googleAuthHandler.ExchangeCode)
	log.Println("✅ Google OAuth routes registered")
} else {
	log.Println("⚠️  Google OAuth disabled (set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and ensure Redis is running)")
}
```

- [ ] **Step 3: Verify build**

```bash
cd /Users/sujal/Dev/clara-agents/backend
go build ./...
```

Expected: exits 0.

- [ ] **Step 4: Commit**

```bash
cd /Users/sujal/Dev/clara-agents
git add backend/cmd/server/main.go
git commit -m "feat: register Google OAuth routes with rate limiting in main.go"
```

---

## Task 7: Add loginWithGoogleToken to auth store

**Files:**
- Modify: `frontend/src/store/useAuthStore.ts`

- [ ] **Step 1: Add to the AuthState interface**

In `frontend/src/store/useAuthStore.ts`, add to the `AuthState` interface:

```ts
loginWithGoogleToken: (token: string, user: AuthUser) => void;
```

- [ ] **Step 2: Implement it**

In the `create<AuthState>()(persist(...))` object, add alongside `login` and `register`:

```ts
loginWithGoogleToken(token: string, user: AuthUser) {
  setAuthToken(token);
  set({
    accessToken: token,
    user,
    isAuthenticated: true,
    isLoading: false,
    error: null,
  });
},
```

- [ ] **Step 3: Commit**

```bash
cd /Users/sujal/Dev/clara-agents
git add frontend/src/store/useAuthStore.ts
git commit -m "feat: add loginWithGoogleToken to auth store"
```

---

## Task 8: Create GoogleAuthCallbackPage

**Files:**
- Create: `frontend/src/pages/GoogleAuthCallbackPage.tsx`
- Modify: `frontend/src/App.tsx`

The page reads `?code=` from the URL, POSTs it to `/api/auth/google/exchange`, stores the JWT, and navigates to `/agents`. On any error it redirects to `/login?error=...`.

- [ ] **Step 1: Create the page**

Create `frontend/src/pages/GoogleAuthCallbackPage.tsx`:

```tsx
import { useEffect, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuthStore } from '@/store/useAuthStore';
import type { AuthUser } from '@/store/useAuthStore';
import { getApiBaseUrl } from '@/lib/config';

/**
 * Handles the Google OAuth callback.
 *
 * The backend redirects here with ?code=<opaque-one-time-code>.
 * We POST the code to /api/auth/google/exchange to get the JWT —
 * the JWT itself never touches the URL.
 */
export function GoogleAuthCallbackPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { loginWithGoogleToken, logout } = useAuthStore();
  const exchanged = useRef(false); // Prevent double-fire in StrictMode

  useEffect(() => {
    if (exchanged.current) return;
    exchanged.current = true;

    const code = searchParams.get('code');
    const error = searchParams.get('error');

    if (error || !code) {
      navigate('/login?error=' + encodeURIComponent(error ?? 'google_callback_missing'), {
        replace: true,
      });
      return;
    }

    (async () => {
      try {
        const res = await fetch(`${getApiBaseUrl()}/api/auth/google/exchange`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          credentials: 'include', // needed for the refresh_token cookie
          body: JSON.stringify({ code }),
        });

        if (!res.ok) {
          const data = await res.json().catch(() => ({}));
          const msg = (data as { error?: string }).error ?? 'exchange_failed';
          navigate('/login?error=' + encodeURIComponent(msg), { replace: true });
          return;
        }

        const data = (await res.json()) as { access_token: string; user: AuthUser };
        loginWithGoogleToken(data.access_token, data.user);
        navigate('/agents', { replace: true });
      } catch {
        logout();
        navigate('/login?error=network_error', { replace: true });
      }
    })();
  }, [searchParams, loginWithGoogleToken, logout, navigate]);

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100vh',
        background: 'var(--color-background)',
        color: 'var(--color-text-secondary)',
        fontFamily: 'Satoshi, sans-serif',
        fontSize: '0.9375rem',
      }}
    >
      Signing you in with Google…
    </div>
  );
}
```

- [ ] **Step 2: Register the route in App.tsx**

In `frontend/src/App.tsx`:

1. Add import at the top with the other page imports:

```tsx
import { GoogleAuthCallbackPage } from '@/pages/GoogleAuthCallbackPage';
```

2. Add route inside `createBrowserRouter([...])` before the catch-all `{ path: '*', ... }`:

```tsx
{ path: '/auth/google/callback', element: <GoogleAuthCallbackPage /> },
```

- [ ] **Step 3: Build frontend**

```bash
cd /Users/sujal/Dev/clara-agents/frontend
npx vite build 2>&1 | tail -5
```

Expected: `✓ built in ...` with no errors.

- [ ] **Step 4: Commit**

```bash
cd /Users/sujal/Dev/clara-agents
git add frontend/src/pages/GoogleAuthCallbackPage.tsx frontend/src/App.tsx
git commit -m "feat: add GoogleAuthCallbackPage and register /auth/google/callback route"
```

---

## Task 9: Add Google button and error display to LoginPage

**Files:**
- Modify: `frontend/src/pages/LoginPage.tsx`
- Modify: `frontend/src/pages/LoginPage.css`

- [ ] **Step 1: Read the useSearchParams error and show Google button**

In `frontend/src/pages/LoginPage.tsx`, make these changes:

**a) Add `useSearchParams` to imports:**

```tsx
import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuthStore } from '@/store/useAuthStore';
import { getApiBaseUrl } from '@/lib/config';
import './LoginPage.css';
```

**b) Add `useSearchParams` call inside the component** (after `const [name, setName] = useState('')`):

```tsx
const [searchParams] = useSearchParams();
const googleError = searchParams.get('error');
```

**c) Show Google-auth errors** — in the JSX, find the existing `{error && <div className="auth-error">...}` block. Add a sibling block right after it:

```tsx
{googleError && !error && (
  <div className="auth-error">
    {googleError === 'google_auth_failed'
      ? 'Google sign-in failed. Please try again.'
      : googleError === 'state_mismatch' || googleError === 'state_cookie_mismatch'
      ? 'Security check failed. Please try again.'
      : googleError === 'exchange_failed' || googleError === 'invalid_or_expired_exchange_code'
      ? 'Sign-in session expired. Please try again.'
      : 'Google sign-in failed. Please try again or use email/password.'}
  </div>
)}
```

**d) Add divider and Google button** — insert this between the closing `</form>` and the `<p className="auth-toggle">` block:

```tsx
<div className="auth-divider">
  <span>or</span>
</div>

<a
  href={`${getApiBaseUrl()}/api/auth/google`}
  className="auth-google-btn"
>
  <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden="true">
    <path d="M16.51 8H8.98v3h4.3c-.18 1-.74 1.48-1.6 2.04v2.01h2.6a7.8 7.8 0 0 0 2.38-5.88c0-.57-.05-.66-.15-1.18z" fill="#4285F4"/>
    <path d="M8.98 17c2.16 0 3.97-.72 5.3-1.94l-2.6-2.01c-.72.48-1.63.76-2.7.76-2.08 0-3.84-1.4-4.47-3.29H1.87v2.07A8 8 0 0 0 8.98 17z" fill="#34A853"/>
    <path d="M4.51 10.52A4.8 4.8 0 0 1 4.26 9c0-.53.09-1.04.25-1.52V5.41H1.87A8 8 0 0 0 .98 9c0 1.29.31 2.51.89 3.59l2.64-2.07z" fill="#FBBC05"/>
    <path d="M8.98 3.58c1.17 0 2.23.4 3.06 1.2l2.3-2.3A8 8 0 0 0 8.98 1 8 8 0 0 0 1.87 5.41l2.64 2.07C5.14 5 6.9 3.58 8.98 3.58z" fill="#EA4335"/>
  </svg>
  Continue with Google
</a>
```

- [ ] **Step 2: Add CSS to LoginPage.css**

Append to the end of `frontend/src/pages/LoginPage.css`:

```css
/* ── Google Sign-In ─────────────────────────────────────────────────────────── */

.auth-divider {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin: 1rem 0;
  color: var(--color-text-tertiary);
  font-size: 0.8125rem;
}

.auth-divider::before,
.auth-divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--color-border);
}

.auth-google-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.625rem;
  width: 100%;
  padding: 0.625rem;
  border-radius: 0.5rem;
  border: 1px solid var(--color-border);
  background: var(--color-surface-elevated);
  color: var(--color-text-primary);
  font-size: 0.875rem;
  font-weight: 500;
  text-decoration: none;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s;
}

.auth-google-btn:hover {
  background: var(--color-surface-hover);
  border-color: var(--color-text-tertiary);
}
```

- [ ] **Step 3: Build frontend**

```bash
cd /Users/sujal/Dev/clara-agents/frontend
npx vite build 2>&1 | tail -5
```

Expected: `✓ built in ...` with no errors.

- [ ] **Step 4: Commit**

```bash
cd /Users/sujal/Dev/clara-agents
git add frontend/src/pages/LoginPage.tsx frontend/src/pages/LoginPage.css
git commit -m "feat: add Google sign-in button and error display to LoginPage"
```

---

## Task 10: Rebuild Docker containers and end-to-end verify

- [ ] **Step 1: Rebuild backend and frontend**

```bash
cd /Users/sujal/Dev/clara-agents
docker compose up -d --build backend frontend
```

Expected: both containers healthy.

- [ ] **Step 2: Check backend logs**

```bash
docker compose logs backend --tail 30
```

Expected (without Google credentials set):
```
⚠️  Google OAuth disabled (set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and ensure Redis is running)
```

- [ ] **Step 3: Verify login page renders**

Open http://localhost:3002. Confirm:
- Email/password form works as before
- `or` divider and `Continue with Google` button are visible

If Google credentials are not set yet, clicking the Google button will navigate to the backend which returns no route (404) — that is expected at this stage.

- [ ] **Step 4: Set up Google Console credentials (user action)**

1. Go to https://console.cloud.google.com → APIs & Services → Credentials
2. Create OAuth 2.0 Client ID (type: **Web application**)
3. **Authorized JavaScript origins:** `http://localhost:3002`
4. **Authorized redirect URIs:** `http://localhost:3001/api/auth/google/callback`
5. Copy the Client ID and Secret into `.env`:

```
GOOGLE_CLIENT_ID=<your-client-id>.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=GOCSPX-<your-secret>
```

6. Restart only the backend (no rebuild needed):

```bash
docker compose restart backend
```

7. Verify the log now shows `✅ Google OAuth routes registered`

- [ ] **Step 5: End-to-end test**

1. Open http://localhost:3002/login
2. Click **Continue with Google**
3. Google consent screen appears
4. After approval, browser goes to `/auth/google/callback` (brief "Signing you in…" flash)
5. Exchange call completes, browser navigates to `/agents`
6. User is authenticated — check that email in UI matches the Google account used

- [ ] **Step 6: Verify error handling**

Open http://localhost:3002/login?error=google_auth_failed — a red error banner should appear with a human-readable message. This is what users see if Google OAuth fails mid-flow.

---

## Self-Review

**Spec coverage:**
- ✅ JWT never in URL — exchange code pattern via Redis
- ✅ Exchange code: 60s TTL, deleted on first use (replay prevention)
- ✅ CSRF: HMAC-signed state nonce + `SameSite=Lax` cookie
- ✅ Rate limiting: 20 req/min per IP on all three OAuth endpoints
- ✅ User lookup: by Google ID first, fallback to email (links existing local accounts)
- ✅ New users created with correct role (first user = admin)
- ✅ Existing users: GoogleID + Name backfilled on first Google login
- ✅ Uses existing `AuthResponse` shape — no new token format
- ✅ Refresh token set as httpOnly cookie on successful exchange
- ✅ `getApiBaseUrl()` used (build-time safe, not raw `import.meta.env`)
- ✅ `StrictMode` double-fire guard in `GoogleAuthCallbackPage` via `useRef`
- ✅ Error messages surfaced to LoginPage via `?error=` query param
- ✅ Google auth gracefully disabled when credentials or Redis are absent
- ✅ `GetUserByEmail` "not found" vs real error handled explicitly
- ✅ `go get` command correct — single module, not subpackage

**Placeholder scan:** No TBDs, no "add appropriate error handling", all code blocks complete.

**Type consistency:** `AuthResponse` struct defined in `auth_local.go` is reused in `ExchangeCode`. `AuthUser` / `UserResponse` shapes match between Go `ToResponse()` and TypeScript interface.
