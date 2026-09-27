package handlers

import (
	"io"
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"

	"kommande/internal/logging"
)

func (h *Handler) ServeImage(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	fileID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	bucket := h.db.GridFSBucket()

	// Get content type from metadata
	var fileInfo bson.M
	_ = h.db.Collection("fs.files").FindOne(r.Context(), bson.M{"_id": fileID}).Decode(&fileInfo)

	contentType := "image/jpeg"
	if metadata, ok := fileInfo["metadata"].(bson.M); ok {
		if ct, ok := metadata["content_type"].(string); ok && ct != "" {
			contentType = ct
		}
	}

	downloadStream, err := bucket.OpenDownloadStream(r.Context(), fileID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = downloadStream.Close() }()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	// A short write means the client hung up mid-image, which is worth seeing
	// because the response is already committed and cannot be retried.
	if n, err := io.Copy(w, downloadStream); err != nil {
		logging.Log.Warn("image stream interrupted", "image_id", fileID.Hex(), "bytes", n, "err", err)
	}
}
