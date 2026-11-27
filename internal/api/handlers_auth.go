package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kevindiu/rclone-manager/internal/domain"
	"github.com/kevindiu/rclone-manager/internal/service"
)

const sessionCookieName = "rclone_manager_session"

type authContextKey struct{}

type authContext struct {
	User      domain.User
	TokenHash string
}

func withAuthContext(ctx context.Context, ac authContext) context.Context {
	return context.WithValue(ctx, authContextKey{}, ac)
}

func authFromContext(ctx context.Context) (authContext, bool) {
	ac, ok := ctx.Value(authContextKey{}).(authContext)
	return ac, ok
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	xfp := r.Header.Get("X-Forwarded-Proto")
	if idx := strings.IndexByte(xfp, ','); idx >= 0 {
		xfp = xfp[:idx]
	}
	return strings.EqualFold(strings.TrimSpace(xfp), "https")
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, rawToken string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    rawToken,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	})
}

func sessionTokenFromRequest(r *http.Request) string {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.auth.Status(r.Context(), sessionTokenFromRequest(r))
	if err != nil {
		s.writeInternalError(w, r, "auth status failed", err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	res, err := s.auth.SetupInitialAdmin(r.Context(), req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrValidation):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrAlreadyInitialized):
			writeError(w, http.StatusForbidden, err.Error())
		default:
			s.writeInternalError(w, r, "initial admin setup failed", err)
		}
		return
	}

	setSessionCookie(w, r, res.RawToken, res.ExpiresAt)
	writeJSON(w, http.StatusCreated, map[string]any{"user": res.User})
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	res, err := s.auth.Login(r.Context(), clientIP(r), req.Username, req.Password)
	if err != nil {
		var rlErr *service.RateLimitError
		switch {
		case errors.As(err, &rlErr):
			secs := max(1, int(rlErr.RetryAfter.Seconds()))
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			writeError(w, http.StatusTooManyRequests, rlErr.Error())
		case errors.Is(err, service.ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			s.writeInternalError(w, r, "login failed", err)
		}
		return
	}

	setSessionCookie(w, r, res.RawToken, res.ExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{"user": res.User})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.auth.Logout(r.Context(), sessionTokenFromRequest(r)); err != nil {
		s.logger.Warn("logout session cleanup failed", "error", err)
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (s *Server) handleAuthChangePassword(w http.ResponseWriter, r *http.Request) {
	ac, ok := authFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := s.auth.ChangePassword(r.Context(), ac.User.ID, ac.TokenHash, req.CurrentPassword, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrValidation):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrInvalidCredentials), errors.Is(err, service.ErrUnauthorized):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			s.writeInternalError(w, r, "change password failed", err)
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password_updated"})
}
