package assets

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestBuildSearchAssetsQueryUsesCombinedFilters(t *testing.T) {
	qText := "ThinkPad"
	managementNumber := "OFS-20250901-0001"
	managementNumberPrefix := "OFS-2025"
	genreCode := "OFS"
	genreName := "Office"
	manufacturer := "Lenovo"
	model := "X1"
	serial := "SN-01"
	owner := "HQ"
	defaultLocation := "Rack"
	location := "Desk"
	lastCheckedBy := "admin"
	notes := "loan"
	name := "Laptop"

	assetID := uint64(11)
	masterID := uint64(7)
	genreID := uint(10)
	managementCategoryID := uint(1)
	statusID := uint(4)
	quantityMin := uint(2)
	quantityMax := uint(8)

	purchasedFrom := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	purchasedTo := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)
	createdFrom := time.Date(2025, time.December, 1, 0, 0, 0, 0, time.UTC)
	createdTo := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	lastCheckedFrom := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	lastCheckedTo := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)

	query, args := buildSearchAssetsQuery(AssetSearchQuery{
		Q:                      &qText,
		ManagementNumber:       &managementNumber,
		ManagementNumberPrefix: &managementNumberPrefix,
		AssetID:                &assetID,
		AssetMasterID:          &masterID,
		GenreID:                &genreID,
		GenreCode:              &genreCode,
		GenreName:              &genreName,
		ManagementCategoryID:   &managementCategoryID,
		Name:                   &name,
		Manufacturer:           &manufacturer,
		Model:                  &model,
		Serial:                 &serial,
		StatusID:               &statusID,
		Owner:                  &owner,
		DefaultLocation:        &defaultLocation,
		Location:               &location,
		PurchasedFrom:          &purchasedFrom,
		PurchasedTo:            &purchasedTo,
		CreatedFrom:            &createdFrom,
		CreatedTo:              &createdTo,
		LastCheckedFrom:        &lastCheckedFrom,
		LastCheckedTo:          &lastCheckedTo,
		LastCheckedBy:          &lastCheckedBy,
		QuantityMin:            &quantityMin,
		QuantityMax:            &quantityMax,
		Notes:                  &notes,
	})

	requiredFragments := []string{
		"LEFT JOIN asset_genres AS g",
		"m.management_number = ?",
		"m.management_number LIKE ? ESCAPE '\\'",
		"a.asset_id = ?",
		"a.asset_master_id = ?",
		"m.genre_id = ?",
		"g.genre_code = ?",
		"g.genre_name LIKE ? ESCAPE '\\'",
		"m.management_category_id = ?",
		"m.name LIKE ? ESCAPE '\\'",
		"m.manufacturer LIKE ? ESCAPE '\\'",
		"COALESCE(m.model, '') LIKE ? ESCAPE '\\'",
		"COALESCE(a.serial, '') LIKE ? ESCAPE '\\'",
		"a.status_id = ?",
		"a.owner LIKE ? ESCAPE '\\'",
		"a.default_location LIKE ? ESCAPE '\\'",
		"COALESCE(a.location, '') LIKE ? ESCAPE '\\'",
		"a.purchased_at >= ?",
		"a.purchased_at < ?",
		"m.created_at >= ?",
		"m.created_at < ?",
		"a.last_checked_at >= ?",
		"a.last_checked_at < ?",
		"COALESCE(a.last_checked_by, '') LIKE ? ESCAPE '\\'",
		"a.quantity >= ?",
		"a.quantity <= ?",
		"COALESCE(a.notes, '') LIKE ? ESCAPE '\\'",
		"ORDER BY m.asset_master_id, a.asset_id",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(query, fragment) {
			t.Fatalf("expected query to contain %q, got:\n%s", fragment, query)
		}
	}

	if len(args) != 31 {
		t.Fatalf("expected 31 args, got %d", len(args))
	}
	if got := args[0]; got != "%ThinkPad%" {
		t.Fatalf("expected q arg pattern, got %#v", got)
	}
	if got := args[5]; got != managementNumber {
		t.Fatalf("expected management number at args[5], got %#v", got)
	}
	if got := args[len(args)-1]; got != "%loan%" {
		t.Fatalf("expected notes arg at tail, got %#v", got)
	}
}

func TestBuildAssetSearchQueryParsesDatesAndNumbers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/assets/search?asset_id=42&quantity_min=3&purchased_to=2026-05-14", nil)

	q, err := buildAssetSearchQuery(c)
	if err != nil {
		t.Fatalf("buildAssetSearchQuery returned error: %v", err)
	}
	if q.AssetID == nil || *q.AssetID != 42 {
		t.Fatalf("expected asset_id 42, got %#v", q.AssetID)
	}
	if q.QuantityMin == nil || *q.QuantityMin != 3 {
		t.Fatalf("expected quantity_min 3, got %#v", q.QuantityMin)
	}
	if q.PurchasedTo == nil {
		t.Fatal("expected purchased_to to be parsed")
	}
	want := time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC)
	if !q.PurchasedTo.Equal(want) {
		t.Fatalf("expected purchased_to %v, got %v", want, *q.PurchasedTo)
	}
}

func TestParseSearchTimeQueryValueRejectsInvalidFormat(t *testing.T) {
	if _, err := parseSearchTimeQueryValue("created_from", "2026/05/14", false); err == nil {
		t.Fatal("expected invalid time format to fail")
	}
}
