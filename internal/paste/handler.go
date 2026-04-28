package paste

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	appmiddleware "github.com/So0ni/clip-pad/internal/middleware"
	"github.com/So0ni/clip-pad/internal/ratelimit"
	"github.com/go-chi/chi/v5"
)

type Renderer interface {
	Render(http.ResponseWriter, int, string, any) error
}

type Handler struct {
	service         *Service
	renderer        Renderer
	maxRequestBytes int64
}

type pageData struct {
	CurrentPage string
	Paste       *Paste
	PasteID     string
	RevealURL   string
	Error       string
	Now         time.Time
}

type createRequest struct {
	Content string `json:"content"`
	Expire  string `json:"expire"`
	Theme   string `json:"theme"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type revealResponse struct {
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func NewHandler(service *Service, renderer Renderer, maxPasteSize int) *Handler {
	maxRequestBytes := int64(maxPasteSize)*4 + 1024
	if maxRequestBytes < int64(maxPasteSize) {
		maxRequestBytes = int64(maxPasteSize)
	}
	return &Handler{service: service, renderer: renderer, maxRequestBytes: maxRequestBytes}
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/p/{id}", h.ShowPaste)
	r.Post("/api/pastes", h.CreatePaste)
	r.Post("/api/pastes/{id}/reveal", h.RevealPaste)
}

func (h *Handler) ShowPaste(w http.ResponseWriter, req *http.Request) {
	id := chi.URLParam(req, "id")
	paste, err := h.service.Get(req.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			h.renderPageError(w, http.StatusNotFound, "notfound.html", pageData{CurrentPage: "paste"})
			return
		}
		log.Printf("show paste error: %v", err)
		h.renderPageError(w, http.StatusInternalServerError, "notfound.html", pageData{CurrentPage: "paste"})
		return
	}

	if paste.BurnAfterRead {
		if err := h.renderer.Render(w, http.StatusOK, "burn.html", pageData{
			CurrentPage: "paste",
			PasteID:     paste.ID,
			RevealURL:   "/api/pastes/" + paste.ID + "/reveal",
		}); err != nil {
			log.Printf("render burn page error: %v", err)
		}
		return
	}

	if err := h.renderer.Render(w, http.StatusOK, "paste.html", pageData{
		CurrentPage: "paste",
		Paste:       paste,
	}); err != nil {
		log.Printf("render paste page error: %v", err)
	}
}

func (h *Handler) CreatePaste(w http.ResponseWriter, req *http.Request) {
	req.Body = http.MaxBytesReader(w, req.Body, h.maxRequestBytes)
	defer req.Body.Close()

	var body createRequest
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if decoder.More() {
		h.writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.service.Create(req.Context(), body.Content, body.Expire, body.Theme, appmiddleware.GetRealIP(req))
	if err != nil {
		switch {
		case errors.Is(err, ErrContentRequired):
			h.writeJSONError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrContentTooLarge):
			h.writeJSONError(w, http.StatusBadRequest, "content exceeds the maximum size")
		case errors.Is(err, ErrInvalidExpireMode):
			h.writeJSONError(w, http.StatusBadRequest, "invalid expire value")
		case errors.Is(err, ratelimit.ErrRateLimitExceeded):
			h.writeJSONError(w, http.StatusTooManyRequests, "rate limit exceeded")
		case errors.Is(err, ErrStorageLimitReached):
			h.writeJSONError(w, http.StatusInsufficientStorage, ErrStorageLimitReached.Error())
		default:
			log.Printf("create paste error: %v", err)
			h.writeJSONError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	h.writeJSON(w, http.StatusCreated, CreateResponse{ID: created.ID, URL: "/p/" + created.ID})
}

func (h *Handler) RevealPaste(w http.ResponseWriter, req *http.Request) {
	id := chi.URLParam(req, "id")
	paste, err := h.service.Reveal(req.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			h.writeJSONError(w, http.StatusNotFound, "notfound")
		case errors.Is(err, ErrNotBurnPaste):
			h.writeJSONError(w, http.StatusBadRequest, "paste is not burn-after-reading")
		default:
			log.Printf("reveal paste error: %v", err)
			h.writeJSONError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, revealResponse{Content: paste.Content, CreatedAt: paste.CreatedAt})
}

func (h *Handler) renderPageError(w http.ResponseWriter, status int, name string, data pageData) {
	if err := h.renderer.Render(w, status, name, data); err != nil {
		http.Error(w, http.StatusText(status), status)
	}
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("encode json error: %v", err)
	}
}

func (h *Handler) writeJSONError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, errorResponse{Error: message})
}
