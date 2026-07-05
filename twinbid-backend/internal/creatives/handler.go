package creatives

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strconv"

	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"

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

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	req, file, header, filename, err := parseCreativeForm(r)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if file != nil {
		defer file.Close()
	}
	item, err := h.svc.Create(r.Context(), auth.UserID(r), chi.URLParam(r, "campaignID"), req, file, header, filename)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, item)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	req, file, header, filename, err := parseCreativeForm(r)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if file != nil {
		defer file.Close()
	}
	item, err := h.svc.Patch(r.Context(), auth.UserID(r), chi.URLParam(r, "id"), req, true, file, header, filename)
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

func parseCreativeForm(r *http.Request) (FormCreativeRequest, multipart.File, *multipart.FileHeader, string, error) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		return FormCreativeRequest{}, nil, nil, "", httpx.BadRequest(err.Error())
	}
	form := r.MultipartForm
	var req FormCreativeRequest
	req.CreativeName = first(form.Value, "creative_name")
	req.Link = first(form.Value, "link")
	if raw := first(form.Value, "trackers_macros"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &req.TrackersMacros); err != nil {
			return req, nil, nil, "", httpx.BadRequest("invalid trackers_macros")
		}
	} else {
		req.TrackersMacros = models.MacroMap{}
	}
	if raw := first(form.Value, "w"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return req, nil, nil, "", httpx.BadRequest("invalid w")
		}
		req.W = &n
	}
	if raw := first(form.Value, "h"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return req, nil, nil, "", httpx.BadRequest("invalid h")
		}
		req.H = &n
	}
	if raw := first(form.Value, "title"); raw != "" {
		req.Title = &raw
	}
	if raw := first(form.Value, "description"); raw != "" {
		req.Description = &raw
	}
	file, header, err := r.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		return req, nil, nil, "", httpx.BadRequest(err.Error())
	}
	filename := first(form.Value, "filename")
	return req, file, header, filename, nil
}

func first(m map[string][]string, key string) string {
	v := m[key]
	if len(v) == 0 {
		return ""
	}
	return v[0]
}
