package server

import (
	"testing"

	"IRIS-backend/internal/platform/db"
)

func TestPrintTemplateRouteIsRegistered(t *testing.T) {
	cfg := &db.Config{Mode: modeRelease}
	router := newRouter(modeRelease, nil, cfg, nil)

	for _, route := range router.Routes() {
		if route.Method == "GET" && route.Path == "/api/v2/assets/print/templates" {
			return
		}
	}

	t.Fatal("expected print template route to be registered")
}
