package creatives

import (
	"io"
	"mime"
	"net/http"
	"strconv"

	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/storage"

	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) ListByCampaign(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListByCampaign(r.Context(), auth.UserID(r), chi.URLParam(r, "campaignID"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) UploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCreativeUploadSize+(1<<20))
	if err := r.ParseMultipartForm(maxCreativeUploadSize); err != nil {
		httpx.Error(w, httpx.BadRequest("invalid multipart creative media upload: "+err.Error()))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, httpx.BadRequest("file is required"))
		return
	}
	defer file.Close()

	image, err := h.svc.UploadImage(
		r.Context(),
		auth.UserID(r),
		chi.URLParam(r, "campaignID"),
		file,
		header,
		r.FormValue("filename"),
	)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, image)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateCreativeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	item, err := h.svc.Create(r.Context(), auth.UserID(r), chi.URLParam(r, "campaignID"), req)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, item)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	var req PatchCreativeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	item, err := h.svc.Patch(r.Context(), auth.UserID(r), chi.URLParam(r, "id"), req)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), auth.UserID(r), chi.URLParam(r, "id")); err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.NoContent(w)
}

// Media serves a private MinIO object through the backend using a permanent UUID URL.
// The route is intentionally public because the URL is embedded into advertising ADM.
func (h *Handler) Media(w http.ResponseWriter, r *http.Request) {
	imageID := chi.URLParam(r, "imageID")
	if r.Method == http.MethodHead {
		image, metadata, err := h.svc.HeadMediaImage(r.Context(), imageID)
		if err != nil {
			httpx.Error(w, err)
			return
		}
		setMediaHeaders(w, image.OriginalName, image.MimeType, metadata)
		w.WriteHeader(http.StatusOK)
		return
	}

	image, object, err := h.svc.GetMediaImage(r.Context(), imageID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	defer object.Body.Close()
	setMediaHeaders(w, image.OriginalName, image.MimeType, object.ObjectMetadata)
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, object.Body); err != nil {
		return
	}
}

func setMediaHeaders(w http.ResponseWriter, filename, storedMimeType string, metadata storage.ObjectMetadata) {
	contentType := metadata.ContentType
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = storedMimeType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": filename}))
	if metadata.ContentLength >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(metadata.ContentLength, 10))
	}
	if metadata.ETag != "" {
		w.Header().Set("ETag", metadata.ETag)
	}
	if metadata.LastModified != nil {
		w.Header().Set("Last-Modified", metadata.LastModified.UTC().Format(http.TimeFormat))
	}
}
