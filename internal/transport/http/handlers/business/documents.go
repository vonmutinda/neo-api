package business

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/internal/transport/http/middleware"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type DocumentHandler struct {
	docRepo repository.BusinessDocumentRepository
}

func NewDocumentHandler(docRepo repository.BusinessDocumentRepository) *DocumentHandler {
	return &DocumentHandler{docRepo: docRepo}
}

func (h *DocumentHandler) Create(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())
	var req struct {
		Name          string   `json:"name"`
		DocumentType  string   `json:"documentType"`
		FileKey       string   `json:"fileKey"`
		FileSizeBytes int64    `json:"fileSizeBytes"`
		MimeType      string   `json:"mimeType"`
		Description   string   `json:"description,omitempty"`
		Tags          []string `json:"tags,omitempty"`
		ExpiresAt     string   `json:"expiresAt,omitempty"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	doc := &domain.BusinessDocument{
		BusinessID:    biz.ID,
		Name:          req.Name,
		DocumentType:  domain.DocumentType(req.DocumentType),
		FileKey:       req.FileKey,
		FileSizeBytes: req.FileSizeBytes,
		MimeType:      req.MimeType,
		UploadedBy:    userID,
		Description:   strPtrOrNil(req.Description),
		Tags:          req.Tags,
		ExpiresAt:     strPtrOrNil(req.ExpiresAt),
	}
	if err := h.docRepo.Create(r.Context(), doc); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, doc)
}

func (h *DocumentHandler) List(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	p := httputil.ParsePagination(r)
	docType := r.URL.Query().Get("type")
	var dtPtr *string
	if docType != "" {
		dtPtr = &docType
	}
	docs, err := h.docRepo.ListByBusiness(r.Context(), biz.ID, dtPtr, p.Limit, p.Offset)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, docs)
}

func (h *DocumentHandler) Get(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	doc, err := h.docRepo.GetByID(r.Context(), docID)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, doc)
}

func (h *DocumentHandler) Update(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	var req struct {
		Name        string   `json:"name,omitempty"`
		Description string   `json:"description,omitempty"`
		Tags        []string `json:"tags,omitempty"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	doc := &domain.BusinessDocument{ID: docID, Name: req.Name, Description: strPtrOrNil(req.Description), Tags: req.Tags}
	if err := h.docRepo.Update(r.Context(), doc); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, doc)
}

func (h *DocumentHandler) Archive(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	if err := h.docRepo.Archive(r.Context(), docID); err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DocumentHandler) ListExpiring(w http.ResponseWriter, r *http.Request) {
	biz := middleware.BusinessFromContext(r.Context())
	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}
	docs, err := h.docRepo.ListExpiring(r.Context(), biz.ID, days)
	if err != nil {
		httputil.HandleError(w, r, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, docs)
}

func (h *DocumentHandler) GetUploadURL(w http.ResponseWriter, r *http.Request) {
	fileKey := "documents/" + time.Now().Format("2006/01/02") + "/" + "file"
	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"uploadUrl": "https://storage.example.com/upload?key=" + fileKey,
		"fileKey":   fileKey,
	})
}
