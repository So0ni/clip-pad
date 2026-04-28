package paste

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appmiddleware "github.com/So0ni/clip-pad/internal/middleware"
	"github.com/go-chi/chi/v5"
)

type testRenderer struct{}

func (testRenderer) Render(w http.ResponseWriter, status int, name string, data any) error {
	w.WriteHeader(status)
	switch name {
	case "burn.html":
		_, _ = w.Write([]byte("burn page"))
	case "paste.html":
		payload := data.(pageData)
		_, _ = w.Write([]byte(payload.Paste.Content))
	case "notfound.html":
		_, _ = w.Write([]byte("notfound"))
	}
	return nil
}

func TestBurnPageDoesNotShowContentAndRevealDeletes(t *testing.T) {
	service := newTestService(t, defaultTestConfig())
	handler := NewHandler(service, testRenderer{}, 1024)
	router := chi.NewRouter()
	resolver := appmiddleware.NewRealIPResolver(false, false, nil)
	router.Use(resolver.Middleware)
	handler.Routes(router)

	created, err := service.Create(context.Background(), "secret text", ExpireBurn, "127.0.0.1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/p/"+created.ID, nil)
	getReq.RemoteAddr = "127.0.0.1:1234"
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)
	if strings.Contains(getResp.Body.String(), "secret text") {
		t.Fatalf("burn page leaked content: %q", getResp.Body.String())
	}

	revealReq := httptest.NewRequest(http.MethodPost, "/api/pastes/"+created.ID+"/reveal", nil)
	revealReq.RemoteAddr = "127.0.0.1:1234"
	revealResp := httptest.NewRecorder()
	router.ServeHTTP(revealResp, revealReq)
	if !strings.Contains(revealResp.Body.String(), "secret text") {
		t.Fatalf("reveal response = %q, want secret text", revealResp.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/p/"+created.ID, nil)
	secondReq.RemoteAddr = "127.0.0.1:1234"
	secondResp := httptest.NewRecorder()
	router.ServeHTTP(secondResp, secondReq)
	if secondResp.Code != http.StatusNotFound {
		t.Fatalf("second GET status = %d, want %d", secondResp.Code, http.StatusNotFound)
	}
	if strings.TrimSpace(secondResp.Body.String()) != "notfound" {
		t.Fatalf("second GET body = %q, want notfound", secondResp.Body.String())
	}
}
