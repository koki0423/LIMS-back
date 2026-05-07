package inventory

import (
	"errors"
	"testing"
)

func TestComputeDisposalPlanSplitsAcrossRows(t *testing.T) {
	rows := []LockedAssetRow{
		{AssetID: 10, Quantity: 2},
		{AssetID: 11, Quantity: 3},
		{AssetID: 12, Quantity: 1},
	}

	got, err := ComputeDisposalPlan(rows, 4)
	if err != nil {
		t.Fatalf("ComputeDisposalPlan returned error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 adjustments, got %d", len(got))
	}
	if got[0].AssetID != 10 || got[0].Delta != -2 {
		t.Fatalf("unexpected first adjustment: %+v", got[0])
	}
	if got[1].AssetID != 11 || got[1].Delta != -2 {
		t.Fatalf("unexpected second adjustment: %+v", got[1])
	}
}

func TestComputeDisposalPlanRejectsInsufficientStock(t *testing.T) {
	rows := []LockedAssetRow{
		{AssetID: 10, Quantity: 1},
		{AssetID: 11, Quantity: 1},
	}

	_, err := ComputeDisposalPlan(rows, 3)
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
}

func TestDetermineStatus(t *testing.T) {
	cases := []struct {
		name                 string
		totalQty             int
		outstandingQty       int
		managementCategoryID int
		want                 int
	}{
		{name: "zero stock", totalQty: 0, outstandingQty: 0, managementCategoryID: 1, want: StatusZeroStock},
		{name: "lend outstanding", totalQty: 3, outstandingQty: 1, managementCategoryID: ManagementCategoryLend, want: StatusLent},
		{name: "normal", totalQty: 3, outstandingQty: 0, managementCategoryID: ManagementCategoryLend, want: StatusNormal},
		{name: "non lend category stays normal", totalQty: 3, outstandingQty: 2, managementCategoryID: 2, want: StatusNormal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetermineStatus(tc.totalQty, tc.outstandingQty, tc.managementCategoryID); got != tc.want {
				t.Fatalf("DetermineStatus() = %d, want %d", got, tc.want)
			}
		})
	}
}
