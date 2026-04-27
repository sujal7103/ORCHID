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
	AccessToken  string              `json:"access_token"`
	RefreshToken string              `json:"refresh_token"`
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
	if err != nil {
		log.Printf("❌ Google userinfo request failed: err=%v", err)
		return errRedirect("userinfo_failed")
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ Google userinfo bad status: %d", resp.StatusCode)
		resp.Body.Close()
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
