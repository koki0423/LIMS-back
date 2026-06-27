package computers

import (
	"database/sql"
	"testing"
	"time"
)

func TestScanComputerPartIncludesActivePartType(t *testing.T) {
	createdAt := time.Date(2026, time.June, 27, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.June, 27, 11, 0, 0, 0, time.UTC)

	item, err := scanComputerPart(scannerFunc(func(dest ...any) error {
		*(dest[0].(*uint64)) = 21
		*(dest[1].(*uint64)) = 301
		*(dest[2].(*string)) = "OFS-20260627-00021"
		*(dest[3].(*string)) = "RTX 4060"
		*(dest[4].(*uint)) = 1
		*(dest[5].(*string)) = "in_use"
		*(dest[6].(*string)) = "使用中"
		*(dest[7].(*sql.NullInt64)) = sql.NullInt64{Int64: 2, Valid: true}
		*(dest[8].(*sql.NullString)) = sql.NullString{String: "gpu", Valid: true}
		*(dest[9].(*sql.NullString)) = sql.NullString{String: "GPU", Valid: true}
		*(dest[10].(*sql.NullString)) = sql.NullString{String: "8GB GDDR6", Valid: true}
		*(dest[11].(*sql.NullString)) = sql.NullString{String: "inventory", Valid: true}
		*(dest[12].(*time.Time)) = createdAt
		*(dest[13].(*time.Time)) = updatedAt
		return nil
	}))
	if err != nil {
		t.Fatalf("scanComputerPart returned error: %v", err)
	}

	if item.ActivePartTypeID == nil || *item.ActivePartTypeID != 2 {
		t.Fatalf("expected active part type id 2, got %#v", item.ActivePartTypeID)
	}
	if item.ActivePartTypeName == nil || *item.ActivePartTypeName != "gpu" {
		t.Fatalf("expected active part type name gpu, got %#v", item.ActivePartTypeName)
	}
	if item.ActivePartTypeDisplayName == nil || *item.ActivePartTypeDisplayName != "GPU" {
		t.Fatalf("expected active part type display name GPU, got %#v", item.ActivePartTypeDisplayName)
	}
}

func TestScanComputerPartLeavesActivePartTypeNilWhenNoActiveConfiguration(t *testing.T) {
	createdAt := time.Date(2026, time.June, 27, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.June, 27, 11, 0, 0, 0, time.UTC)

	item, err := scanComputerPart(scannerFunc(func(dest ...any) error {
		*(dest[0].(*uint64)) = 22
		*(dest[1].(*uint64)) = 302
		*(dest[2].(*string)) = "OFS-20260627-00022"
		*(dest[3].(*string)) = "Memory Module"
		*(dest[4].(*uint)) = 2
		*(dest[5].(*string)) = "stock"
		*(dest[6].(*string)) = "在庫"
		*(dest[7].(*sql.NullInt64)) = sql.NullInt64{}
		*(dest[8].(*sql.NullString)) = sql.NullString{}
		*(dest[9].(*sql.NullString)) = sql.NullString{}
		*(dest[10].(*sql.NullString)) = sql.NullString{}
		*(dest[11].(*sql.NullString)) = sql.NullString{}
		*(dest[12].(*time.Time)) = createdAt
		*(dest[13].(*time.Time)) = updatedAt
		return nil
	}))
	if err != nil {
		t.Fatalf("scanComputerPart returned error: %v", err)
	}

	if item.ActivePartTypeID != nil {
		t.Fatalf("expected nil active part type id, got %#v", item.ActivePartTypeID)
	}
	if item.ActivePartTypeName != nil {
		t.Fatalf("expected nil active part type name, got %#v", item.ActivePartTypeName)
	}
	if item.ActivePartTypeDisplayName != nil {
		t.Fatalf("expected nil active part type display name, got %#v", item.ActivePartTypeDisplayName)
	}
}

type scannerFunc func(dest ...any) error

func (f scannerFunc) Scan(dest ...any) error {
	return f(dest...)
}
