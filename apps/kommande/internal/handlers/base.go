package handlers

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"kommande/internal/config"
	"kommande/internal/logging"
	"kommande/internal/middleware"
	"kommande/internal/models"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Handler struct {
	db           *mongo.Database
	files        embed.FS
	cfg          *config.Config
	funcMap      template.FuncMap
	oauth2Config *oauth2.Config
	oidcVerifier *oidc.IDTokenVerifier
}

func New(db *mongo.Database, files embed.FS, cfg *config.Config) (*Handler, error) {
	provider, err := oidc.NewProvider(context.Background(), cfg.OIDCIssuer)
	if err != nil {
		return nil, fmt.Errorf("OIDC provider init: %w", err)
	}
	oauth2Cfg := &oauth2.Config{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.OIDCRedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.OIDCClientID})
	logging.Log.Info("oidc provider discovered", "issuer", cfg.OIDCIssuer,
		"auth_endpoint", provider.Endpoint().AuthURL,
		"token_endpoint", provider.Endpoint().TokenURL)

	h := &Handler{
		db:           db,
		files:        files,
		cfg:          cfg,
		oauth2Config: oauth2Cfg,
		oidcVerifier: verifier,
	}
	h.funcMap = template.FuncMap{
		"statusClass": statusClass,
		"statusLabel": statusLabel,
		"formatQty":   formatQty,
		"initial":     initial,
		"quantityFor": quantityFor,
		"imageURL":    imageURL,
		"unitStep":    unitStep,
		"containsID": func(ids []bson.ObjectID, id bson.ObjectID) bool {
			for _, oid := range ids {
				if oid == id {
					return true
				}
			}
			return false
		},
	}
	return h, nil
}

type PageData struct {
	Title     string
	User      *models.User
	Flash     string
	FlashType string
	Data      interface{}
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, tmplPath, title string, data interface{}) {
	user := middleware.GetUser(r.Context())
	flash, flashType := getFlash(w, r)

	pd := PageData{
		Title:     title,
		User:      user,
		Flash:     flash,
		FlashType: flashType,
		Data:      data,
	}

	t, err := template.New("layout").Funcs(h.funcMap).ParseFS(h.files,
		"templates/layout.html",
		tmplPath,
	)
	if err != nil {
		logging.Log.Error("template parse error", "template", tmplPath, "err", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", pd); err != nil {
		// The status line is already sent at this point, so the response cannot
		// be turned into a 500; log the template that failed and what it rendered.
		logging.Log.Error("template exec error", "template", tmplPath, "title", title,
			"user_id", middleware.UserID(user), "err", err)
	}
}

func setFlash(w http.ResponseWriter, msg, msgType string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "flash",
		Value:    url.QueryEscape(msg),
		Path:     "/",
		MaxAge:   60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "flash_type",
		Value:    msgType,
		Path:     "/",
		MaxAge:   60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func getFlash(w http.ResponseWriter, r *http.Request) (string, string) {
	fc, err := r.Cookie("flash")
	if err != nil {
		return "", "info"
	}
	tc, _ := r.Cookie("flash_type")

	http.SetCookie(w, &http.Cookie{Name: "flash", Value: "", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: "flash_type", Value: "", Path: "/", MaxAge: -1})

	msg, _ := url.QueryUnescape(fc.Value)
	ft := "info"
	if tc != nil {
		ft = tc.Value
	}
	return msg, ft
}

func statusClass(status string) string {
	switch status {
	case "pending":
		return "badge-warning"
	case "confirmed":
		return "badge-primary"
	case "delivered":
		return "badge-success"
	case "cancelled":
		return "badge-danger"
	}
	return "badge-secondary"
}

func statusLabel(status string) string {
	switch status {
	case "pending":
		return "En attente"
	case "confirmed":
		return "Confirmée"
	case "delivered":
		return "Livrée"
	case "cancelled":
		return "Annulée"
	}
	return status
}

func formatQty(q float64) string {
	if q == float64(int64(q)) {
		return fmt.Sprintf("%d", int64(q))
	}
	s := fmt.Sprintf("%.2f", q)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

func initial(name string) string {
	if name == "" {
		return "?"
	}
	r := []rune(name)
	return strings.ToUpper(string(r[0]))
}

func imageURL(id *bson.ObjectID) string {
	if id == nil {
		return ""
	}
	return id.Hex()
}

func unitStep(unit string) string {
	switch unit {
	case "kg", "g", "L", "cL":
		return "0.5"
	default:
		return "1"
	}
}

// groupByCategory organises articles into category groups (alphabetical).
// Articles with no category appear in a trailing "Autres" group.
func groupByCategory(cats []models.Category, articles []models.Article) []models.ArticleGroup {
	catIndex := make(map[bson.ObjectID]int, len(cats))
	groups := make([]models.ArticleGroup, len(cats))
	for i, c := range cats {
		c2 := c
		groups[i] = models.ArticleGroup{Category: &c2}
		catIndex[c.ID] = i
	}

	var uncategorised []models.Article
	for _, a := range articles {
		placed := false
		for _, cid := range a.CategoryIDs {
			if idx, ok := catIndex[cid]; ok {
				groups[idx].Articles = append(groups[idx].Articles, a)
				placed = true
			}
		}
		if !placed {
			uncategorised = append(uncategorised, a)
		}
	}

	// Remove empty category groups
	var result []models.ArticleGroup
	for _, g := range groups {
		if len(g.Articles) > 0 {
			result = append(result, g)
		}
	}
	if len(uncategorised) > 0 {
		result = append(result, models.ArticleGroup{Articles: uncategorised})
	}
	return result
}

func quantityFor(order *models.Order, articleID bson.ObjectID) string {
	if order == nil {
		return "0"
	}
	for _, item := range order.Items {
		if item.ArticleID == articleID {
			return formatQty(item.Quantity)
		}
	}
	return "0"
}
