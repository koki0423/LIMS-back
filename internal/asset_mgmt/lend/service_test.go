package lend

import (
	"testing"
	"time"
)

func TestParseDueOnUTC(t *testing.T) {
	raw := "2026-05-07"

	got, ok, err := parseDueOnUTC(&raw)
	if err != nil {
		t.Fatalf("parseDueOnUTC returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %v", got.Location())
	}
	if got.Year() != 2026 || got.Month() != time.May || got.Day() != 7 {
		t.Fatalf("unexpected parsed date: %v", got)
	}
}

func TestParseDueOnUTCRejectsInvalidFormat(t *testing.T) {
	raw := "2026/05/07"

	_, _, err := parseDueOnUTC(&raw)
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}
