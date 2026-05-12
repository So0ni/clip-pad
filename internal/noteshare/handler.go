package noteshare

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

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
	Share       *NoteShare
	Error       string
}

type createRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Expire  string `json:"expire"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewHandler(service *Service, renderer Renderer, maxContentSize int) *Handler {
	maxRequestBytes := int64(maxContentSize)*4 + 2048
	if maxRequestBytes < int64(maxContentSize) {
		maxRequestBytes = int64(maxContentSize)
	}
	return &Handler{service: service, renderer: renderer, maxRequestBytes: maxRequestBytes}
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/n/{id}", h.ShowNoteShare)
	r.Post("/api/notepad/shares", h.CreateNoteShare)
}

func (h *Handler) ShowNoteShare(w http.ResponseWriter, req *http.Request) {
	id := chi.URLParam(req, "id")
	share, err := h.service.Get(req.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			h.renderPageError(w, http.StatusNotFound, "notfound.html", pageData{CurrentPage: "note-share"})
			return
		}
		log.Printf("show note share error: %v", err)
		h.renderPageError(w, http.StatusInternalServerError, "notfound.html", pageData{CurrentPage: "note-share"})
		return
	}

	if err := h.renderer.Render(w, http.StatusOK, "note_share.html", pageData{
		CurrentPage: "note-share",
		Share:       share,
	}); err != nil {
		log.Printf("render note share page error: %v", err)
	}
}

func (h *Handler) CreateNoteShare(w http.ResponseWriter, req *http.Request) {
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

	created, err := h.service.Create(req.Context(), body.Title, body.Content, body.Expire, appmiddleware.GetRealIP(req))
	if err != nil {
		switch {
		case errors.Is(err, ErrContentRequired):
			h.writeJSONError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrTitleTooLarge):
			h.writeJSONError(w, http.StatusBadRequest, "title exceeds the maximum size")
		case errors.Is(err, ErrContentTooLarge):
			h.writeJSONError(w, http.StatusBadRequest, "content exceeds the maximum size")
		case errors.Is(err, ErrInvalidExpireMode):
			h.writeJSONError(w, http.StatusBadRequest, "invalid expire value")
		case errors.Is(err, ratelimit.ErrRateLimitExceeded):
			h.writeJSONError(w, http.StatusTooManyRequests, "rate limit exceeded")
		case errors.Is(err, ErrStorageLimitReached):
			h.writeJSONError(w, http.StatusInsufficientStorage, ErrStorageLimitReached.Error())
		default:
			log.Printf("create note share error: %v", err)
			h.writeJSONError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	h.writeJSON(w, http.StatusCreated, CreateResponse{ID: created.ID, URL: "/n/" + created.ID, ExpiresAt: created.ExpiresAt})
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
