package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"IRIS-backend/internal/platform/db"
)

func TestPrintTemplateRouteIsRegistered(t *testing.T) {
	cfg := &db.Config{Mode: modeRelease}
	router := newRouter(modeRelease, nil, cfg)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v2/assets/print/templates?width=9&type=qrcode",
		nil,
	)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Fatalf("expected print template route to be registered, got %d", rec.Code)
	}
}
