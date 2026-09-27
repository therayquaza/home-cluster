package middleware

import (
	"context"
	"errors"
	"net/http"

	"kommande/internal/logging"
	"kommande/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type contextKey string

const UserContextKey contextKey = "user"

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	PhotoID  string `json:"photo_id,omitempty"`
	jwt.RegisteredClaims
}

func RequireAuth(jwtSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := extractUser(r, jwtSecret)
		if err != nil {
			clearAuthCookie(w)
			// A visitor arriving without a cookie is the normal entry point to
			// the login flow; a rejected token is not. Split the two so a busy
			// login page does not look like an auth failure spike.
			if errors.Is(err, http.ErrNoCookie) {
				logging.Log.Debug("auth redirect to login", "method", r.Method, "path", r.URL.Path, "reason", "no_session")
			} else {
				logging.Log.Warn("auth token rejected", "method", r.Method, "path", r.URL.Path, "err", err)
			}
			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireAdmin(jwtSecret string, next http.Handler) http.Handler {
	return RequireAuth(jwtSecret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		if user == nil || user.Role != "admin" {
			logging.Log.Warn("admin access denied", "method", r.Method, "path", r.URL.Path,
				"user_id", UserID(user), "role", Role(user))
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// UserID returns a user id suitable for logs, tolerating a nil user.
func UserID(u *models.User) string {
	if u == nil {
		return "<none>"
	}
	return u.ID.Hex()
}

// Role returns a user role suitable for logs, tolerating a nil user.
func Role(u *models.User) string {
	if u == nil {
		return "<none>"
	}
	return u.Role
}

// Username returns a username suitable for logs, tolerating a nil user.
func Username(u *models.User) string {
	if u == nil {
		return "<none>"
	}
	return u.Username
}

func GetUser(ctx context.Context) *models.User {
	u, _ := ctx.Value(UserContextKey).(*models.User)
	return u
}

func extractUser(r *http.Request, jwtSecret string) (*models.User, error) {
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		return nil, err
	}

	claims := &Claims{}
	_, err = jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}

	id, _ := bson.ObjectIDFromHex(claims.UserID)
	user := &models.User{
		ID:       id,
		Username: claims.Username,
		Role:     claims.Role,
	}
	if claims.PhotoID != "" {
		pid, err := bson.ObjectIDFromHex(claims.PhotoID)
		if err == nil {
			user.PhotoID = &pid
		}
	}
	return user, nil
}

func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   "auth_token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}
