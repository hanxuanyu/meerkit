package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"meerkit/internal/auth"
)

const authSessionKey = "meerkit.auth.session"
const authPrincipalKey = "meerkit.auth.principal"

type loginAttempt struct {
	count   int
	resetAt time.Time
}
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

func newLoginLimiter() *loginLimiter { return &loginLimiter{attempts: make(map[string]loginAttempt)} }
func (l *loginLimiter) allowed(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	value := l.attempts[key]
	if now.After(value.resetAt) {
		delete(l.attempts, key)
		return true
	}
	return value.count < 8
}
func (l *loginLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	value := l.attempts[key]
	if now.After(value.resetAt) {
		value = loginAttempt{resetAt: now.Add(10 * time.Minute)}
	}
	value.count++
	l.attempts[key] = value
}
func (l *loginLimiter) clear(key string) { l.mu.Lock(); delete(l.attempts, key); l.mu.Unlock() }

func (a *APIServer) authStatus(c *gin.Context) {
	initialized, err := a.auth.Initialized(c.Request.Context())
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, map[string]any{"initialized": initialized})
}
func (a *APIServer) authSetup(c *gin.Context) {
	var payload struct {
		AccessKey string `json:"access_key"`
		Confirm   string `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if payload.AccessKey != payload.Confirm {
		writeError(c.Writer, http.StatusBadRequest, "validation_error", "access key confirmation does not match")
		return
	}
	session, err := a.auth.Setup(c.Request.Context(), payload.AccessKey)
	if err != nil {
		writeError(c.Writer, http.StatusBadRequest, "setup_failed", err.Error())
		return
	}
	setSessionCookie(c, session)
	writeJSON(c.Writer, http.StatusCreated, session)
}
func (a *APIServer) authLogin(c *gin.Context) {
	key := c.ClientIP()
	if !a.loginLimiter.allowed(key) {
		writeError(c.Writer, http.StatusTooManyRequests, "rate_limited", "too many login attempts; try again later")
		return
	}
	var payload struct {
		AccessKey string `json:"access_key"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	session, err := a.auth.Login(c.Request.Context(), payload.AccessKey)
	if err != nil {
		a.loginLimiter.fail(key)
		writeError(c.Writer, http.StatusUnauthorized, "invalid_credentials", err.Error())
		return
	}
	a.loginLimiter.clear(key)
	setSessionCookie(c, session)
	writeJSON(c.Writer, http.StatusOK, session)
}
func (a *APIServer) authLogout(c *gin.Context) {
	token, _ := c.Cookie(auth.CookieName)
	_ = a.auth.Logout(c.Request.Context(), token)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(auth.CookieName, "", -1, "/", "", c.Request.TLS != nil, true)
	c.Status(http.StatusNoContent)
}

func (a *APIServer) authChangeKey(c *gin.Context) {
	var payload struct {
		CurrentAccessKey string `json:"current_access_key"`
		AccessKey        string `json:"access_key"`
		Confirm          string `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if payload.AccessKey != payload.Confirm {
		writeError(c.Writer, http.StatusBadRequest, "validation_error", "access key confirmation does not match")
		return
	}
	if err := a.auth.ChangeKey(c.Request.Context(), payload.CurrentAccessKey, payload.AccessKey); err != nil {
		writeError(c.Writer, http.StatusBadRequest, "change_key_failed", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *APIServer) authSession(c *gin.Context) {
	value, _ := c.Get(authSessionKey)
	writeJSON(c.Writer, http.StatusOK, value)
}
func (a *APIServer) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if header := c.GetHeader("Authorization"); header != "" {
			parts := strings.Fields(header)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeError(c.Writer, http.StatusUnauthorized, "authentication_required", "invalid bearer token")
				c.Abort()
				return
			}
			principal, err := a.auth.AuthenticateToken(c.Request.Context(), parts[1])
			if err != nil {
				writeError(c.Writer, http.StatusUnauthorized, "authentication_required", err.Error())
				c.Abort()
				return
			}
			required := auth.ScopeAPIRead
			if requiresCSRF(c.Request.Method) {
				required = auth.ScopeAPIWrite
			}
			if !auth.HasScope(principal, required) {
				writeError(c.Writer, http.StatusForbidden, "insufficient_scope", "token scope is insufficient")
				c.Abort()
				return
			}
			c.Set(authPrincipalKey, principal)
			c.Next()
			return
		}
		token, _ := c.Cookie(auth.CookieName)
		session, err := a.auth.Authenticate(c.Request.Context(), token)
		if err != nil {
			writeError(c.Writer, http.StatusUnauthorized, "authentication_required", err.Error())
			c.Abort()
			return
		}
		if requiresCSRF(c.Request.Method) && subtle.ConstantTimeCompare([]byte(c.GetHeader("X-CSRF-Token")), []byte(session.CSRFToken)) != 1 {
			writeError(c.Writer, http.StatusForbidden, "csrf_failed", "missing or invalid CSRF token")
			c.Abort()
			return
		}
		c.Set(authSessionKey, map[string]any{"csrf_token": session.CSRFToken, "expires_at": session.ExpiresAt})
		c.Set(authPrincipalKey, nil)
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(auth.CookieName, token, int(time.Until(session.ExpiresAt).Seconds()), "/", "", c.Request.TLS != nil, true)
		c.Next()
	}
}

func isBearerPrincipal(c *gin.Context) bool {
	value, exists := c.Get(authPrincipalKey)
	return exists && value != nil
}
func requiresCSRF(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}
func setSessionCookie(c *gin.Context, session auth.Session) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(auth.CookieName, session.Token, int(time.Until(session.ExpiresAt).Seconds()), "/", "", c.Request.TLS != nil, true)
}
