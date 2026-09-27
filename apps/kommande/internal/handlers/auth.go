package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"kommande/internal/logging"
	"kommande/internal/middleware"
	"kommande/internal/models"
)

func (h *Handler) OIDCLoginRedirect(w http.ResponseWriter, r *http.Request) {
	if u := middleware.GetUser(r.Context()); u != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	state, err := generateState()
	if err != nil {
		logging.Log.Error("failed to generate OIDC state", "err", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	logging.Log.Info("auth login started", "redirect_to", h.cfg.OIDCIssuer)
	http.Redirect(w, r, h.oauth2Config.AuthCodeURL(state), http.StatusFound)
}

func (h *Handler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if providerErr := q.Get("error"); providerErr != "" {
		desc := q.Get("error_description")
		logging.Log.Warn("oidc provider returned an error", "error", providerErr, "description", desc)
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	cookieState, err := r.Cookie("oidc_state")
	queryState := q.Get("state")
	if err != nil {
		logging.Log.Warn("oidc callback without state cookie", "err", err)
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}
	if cookieState.Value != queryState {
		// Never log either value; only that they disagreed.
		logging.Log.Warn("oidc state mismatch", "cookie_len", len(cookieState.Value), "query_len", len(queryState))
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "oidc_state", Path: "/", MaxAge: -1})

	code := q.Get("code")
	if code == "" {
		logging.Log.Warn("oidc callback without authorization code")
		http.Error(w, "Authentication failed", http.StatusBadRequest)
		return
	}

	oauth2Token, err := h.oauth2Config.Exchange(r.Context(), code)
	if err != nil {
		// invalid_client here almost always means OIDC_CLIENT_SECRET is empty or
		// does not match the client registered in Keycloak.
		logging.Log.Error("oidc code exchange failed", "client_id", h.cfg.OIDCClientID, "err", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		logging.Log.Error("no id_token in token response", "client_id", h.cfg.OIDCClientID)
		http.Error(w, "No ID token in response", http.StatusInternalServerError)
		return
	}
	idToken, err := h.oidcVerifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		logging.Log.Error("oidc id token verification failed", "err", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	var claims struct {
		Email             string   `json:"email"`
		PreferredUsername string   `json:"preferred_username"`
		Groups            []string `json:"groups"`
	}
	if err := idToken.Claims(&claims); err != nil {
		logging.Log.Error("oidc claims extraction failed", "err", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}
	if claims.Email == "" {
		logging.Log.Error("id token has no email claim", "subject", idToken.Subject)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	role := "user"
	for _, g := range claims.Groups {
		if g == adminGroup {
			role = "admin"
			break
		}
	}

	user, err := h.upsertOIDCUser(r.Context(), claims.Email, claims.PreferredUsername, role)
	if err != nil {
		logging.Log.Error("oidc user upsert failed", "err", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	token, err := h.generateJWT(user)
	if err != nil {
		logging.Log.Error("jwt signing failed", "user_id", user.ID.Hex(), "err", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	logging.Log.Info("login succeeded", "user_id", user.ID.Hex(), "username", user.Username,
		"role", role, "admin_group", role == "admin")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// adminGroup is the Keycloak group whose members get the admin role.
const adminGroup = "kommande-admins"

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	http.SetCookie(w, &http.Cookie{
		Name:   "auth_token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	logoutURL := h.cfg.OIDCIssuer + "/protocol/openid-connect/logout" +
		"?post_logout_redirect_uri=" + url.QueryEscape(h.cfg.BaseURL) +
		"&client_id=" + url.QueryEscape(h.cfg.OIDCClientID)
	logging.Log.Info("logout", "user_id", middleware.UserID(user), "username", middleware.Username(user))
	http.Redirect(w, r, logoutURL, http.StatusSeeOther)
}

func (h *Handler) upsertOIDCUser(ctx context.Context, email, username, role string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if username == "" {
		username = email
	}

	filter := bson.M{"email": email}
	update := bson.M{
		"$set": bson.M{
			"username": username,
			"role":     role,
		},
		"$setOnInsert": bson.M{
			"_id":        bson.NewObjectID(),
			"email":      email,
			"created_at": time.Now(),
		},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var user models.User
	err := h.db.Collection("users").FindOneAndUpdate(ctx, filter, update, opts).Decode(&user)
	return &user, err
}

func (h *Handler) generateJWT(user *models.User) (string, error) {
	claims := middleware.Claims{
		UserID:   user.ID.Hex(),
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	if user.PhotoID != nil {
		claims.PhotoID = user.PhotoID.Hex()
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		return "", err
	}
	return signed, nil
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
