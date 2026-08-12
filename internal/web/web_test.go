package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AstonMarty13/gocheck/internal/monitor"
)

func TestHandlerRendersSites(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	m := monitor.New([]string{target.URL}, time.Minute, time.Second)
	m.Check(context.Background(), target.URL)

	rec := httptest.NewRecorder()
	Handler(m).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, target.URL) {
		t.Errorf("body does not mention the monitored URL %q", target.URL)
	}
	if !strings.Contains(body, "UP") {
		t.Error("body does not report the site as UP")
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestHandlerUnknownPathIs404(t *testing.T) {
	m := monitor.New(nil, time.Minute, time.Second)

	rec := httptest.NewRecorder()
	Handler(m).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/does-not-exist", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
