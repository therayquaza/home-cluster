package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"kommande/internal/email"
	"kommande/internal/logging"
	"kommande/internal/middleware"
	"kommande/internal/models"
)

// ─── Dashboard ────────────────────────────────────────────────────────────────

type dashboardData struct {
	UserCount    int64
	ArticleCount int64
	TodayOrders  int64
	PendingCount int64
	RecentOrders []models.Order
}

func (h *Handler) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	today := time.Now().Format("2006-01-02")

	userCount, _ := h.db.Collection("users").CountDocuments(ctx, bson.M{})
	articleCount, _ := h.db.Collection("articles").CountDocuments(ctx, bson.M{})
	todayOrders, _ := h.db.Collection("orders").CountDocuments(ctx, bson.M{"date": today})
	pendingCount, _ := h.db.Collection("orders").CountDocuments(ctx, bson.M{"date": today, "status": "pending"})

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(5)
	cursor, _ := h.db.Collection("orders").Find(ctx, bson.M{"date": today}, opts)
	var recent []models.Order
	if cursor != nil {
		_ = cursor.All(ctx, &recent)
		_ = cursor.Close(ctx)
	}

	h.render(w, r, "templates/admin/dashboard.html", "Tableau de bord", dashboardData{
		UserCount:    userCount,
		ArticleCount: articleCount,
		TodayOrders:  todayOrders,
		PendingCount: pendingCount,
		RecentOrders: recent,
	})
}

// ─── Articles ─────────────────────────────────────────────────────────────────

func (h *Handler) AdminArticles(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	cursor, err := h.db.Collection("articles").Find(ctx, bson.M{})
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = cursor.Close(ctx) }()

	var articles []models.Article
	if err := cursor.All(ctx, &articles); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	h.render(w, r, "templates/admin/articles.html", "Articles", articles)
}

type articleFormData struct {
	Article    *models.Article
	Categories []models.Category
}

func (h *Handler) loadCategories(ctx context.Context) []models.Category {
	opts := options.Find().SetSort(bson.D{{Key: "name", Value: 1}})
	cursor, err := h.db.Collection("categories").Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil
	}
	defer func() { _ = cursor.Close(ctx) }()
	var cats []models.Category
	_ = cursor.All(ctx, &cats)
	return cats
}

func parseCategoryIDs(r *http.Request) []bson.ObjectID {
	var ids []bson.ObjectID
	for _, raw := range r.Form["category_ids"] {
		if id, err := bson.ObjectIDFromHex(raw); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func (h *Handler) AdminNewArticle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	h.render(w, r, "templates/admin/article-form.html", "Nouvel article", articleFormData{
		Categories: h.loadCategories(ctx),
	})
}

func (h *Handler) AdminCreateArticle(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	description := r.FormValue("description")
	unit := r.FormValue("unit")
	available := r.FormValue("available") == "on"

	if name == "" || unit == "" {
		setFlash(w, "Nom et unité sont obligatoires.", "danger")
		http.Redirect(w, r, "/admin/articles/new", http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	now := time.Now()
	article := models.Article{
		ID:          bson.NewObjectID(),
		Name:        name,
		Description: description,
		Unit:        unit,
		Available:   available,
		CategoryIDs: parseCategoryIDs(r),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	imageID, err := h.uploadImage(ctx, r)
	if err == nil {
		article.ImageID = &imageID
	}

	if _, err := h.db.Collection("articles").InsertOne(ctx, article); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	setFlash(w, "Article créé avec succès.", "success")
	http.Redirect(w, r, "/admin/articles", http.StatusSeeOther)
}

func (h *Handler) AdminEditArticle(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var article models.Article
	if err := h.db.Collection("articles").FindOne(ctx, bson.M{"_id": id}).Decode(&article); err != nil {
		http.NotFound(w, r)
		return
	}

	h.render(w, r, "templates/admin/article-form.html", "Modifier "+article.Name, articleFormData{
		Article:    &article,
		Categories: h.loadCategories(ctx),
	})
}

func (h *Handler) AdminUpdateArticle(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	description := r.FormValue("description")
	unit := r.FormValue("unit")
	available := r.FormValue("available") == "on"

	if name == "" || unit == "" {
		setFlash(w, "Nom et unité sont obligatoires.", "danger")
		http.Redirect(w, r, "/admin/articles/"+idStr+"/edit", http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	update := bson.M{
		"name":         name,
		"description":  description,
		"unit":         unit,
		"available":    available,
		"category_ids": parseCategoryIDs(r),
		"updated_at":   time.Now(),
	}

	imageID, err := h.uploadImage(ctx, r)
	if err == nil {
		update["image_id"] = imageID
	}

	if _, err := h.db.Collection("articles").UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": update},
	); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	setFlash(w, "Article mis à jour.", "success")
	http.Redirect(w, r, "/admin/articles", http.StatusSeeOther)
}

func (h *Handler) AdminDeleteArticle(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	actor := middleware.GetUser(r.Context())
	res, err := h.db.Collection("articles").DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		logging.Log.Error("article delete failed", "article_id", id.Hex(), "actor_id", middleware.UserID(actor), "err", err)
		setFlash(w, "Erreur lors de la suppression.", "danger")
		http.Redirect(w, r, "/admin/articles", http.StatusSeeOther)
		return
	}
	if res.DeletedCount == 0 {
		logging.Log.Warn("article delete matched nothing", "article_id", id.Hex(), "actor_id", middleware.UserID(actor))
	}
	logging.Log.Info("article deleted", "article_id", id.Hex(), "actor_id", middleware.UserID(actor))

	setFlash(w, "Article supprimé.", "success")
	http.Redirect(w, r, "/admin/articles", http.StatusSeeOther)
}

// ─── Orders ───────────────────────────────────────────────────────────────────

type adminOrdersData struct {
	Orders []models.Order
	Date   string
}

func (h *Handler) AdminOrders(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	cursor, err := h.db.Collection("orders").Find(ctx, bson.M{"date": date}, opts)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = cursor.Close(ctx) }()

	var orders []models.Order
	if err := cursor.All(ctx, &orders); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	h.render(w, r, "templates/admin/orders.html", "Commands du jour", adminOrdersData{
		Orders: orders,
		Date:   date,
	})
}

func (h *Handler) AdminRespondOrder(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	action := r.FormValue("action") // confirm, deliver, cancel
	note := r.FormValue("note")

	newStatus := ""
	switch action {
	case "confirm":
		newStatus = "confirmed"
	case "deliver":
		newStatus = "delivered"
	case "cancel":
		newStatus = "cancelled"
	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var order models.Order
	if err := h.db.Collection("orders").FindOne(ctx, bson.M{"_id": id}).Decode(&order); err != nil {
		http.NotFound(w, r)
		return
	}

	_, err = h.db.Collection("orders").UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"status":     newStatus,
			"admin_note": note,
			"updated_at": time.Now(),
		}},
	)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if newStatus == "delivered" {
		h.notifyClientDelivered(ctx, order, note)
	}

	date := r.FormValue("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	setFlash(w, "Commande mise à jour.", "success")
	http.Redirect(w, r, "/admin/orders?date="+date, http.StatusSeeOther)
}

func (h *Handler) notifyClientDelivered(ctx context.Context, order models.Order, message string) {
	var user models.User
	if err := h.db.Collection("users").FindOne(ctx, bson.M{"username": order.Username}).Decode(&user); err != nil || user.Email == "" {
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Bonjour %s,\n\nVotre commande du %s a été livrée.\n\n", order.Username, order.Date)
	if message != "" {
		fmt.Fprintf(&sb, "Message de l'équipe :\n%s\n\n", message)
	}
	sb.WriteString("Détail de votre commande :\n")
	for _, item := range order.Items {
		fmt.Fprintf(&sb, "  - %s : %s %s\n", item.ArticleName, formatQty(item.Quantity), item.Unit)
	}
	sb.WriteString("\nMerci de votre confiance !\n")

	email.Send(h.cfg, user.Email, fmt.Sprintf("[Kommande] Votre commande du %s a été livrée", order.Date), sb.String())
}

// ─── Image upload helper ──────────────────────────────────────────────────────

func (h *Handler) uploadImage(ctx context.Context, r *http.Request) (bson.ObjectID, error) {
	file, header, err := r.FormFile("image")
	if err != nil {
		return bson.NilObjectID, err
	}
	defer func() { _ = file.Close() }()

	bucket := h.db.GridFSBucket()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	uploadStream, err := bucket.OpenUploadStream(
		ctx,
		header.Filename,
		options.GridFSUpload().SetMetadata(bson.D{{Key: "content_type", Value: contentType}}),
	)
	if err != nil {
		return bson.NilObjectID, err
	}
	defer func() { _ = uploadStream.Close() }()

	if _, err := io.Copy(uploadStream, file); err != nil {
		return bson.NilObjectID, err
	}

	fileID, ok := uploadStream.FileID.(bson.ObjectID)
	if !ok {
		return bson.NilObjectID, nil
	}
	return fileID, nil
}
