package computers

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestCreateComputerConfigurationRejectsInvalidDateOrder(t *testing.T) {
	store := &fakeComputerStore{
		assetExists:    true,
		partTypeExists: true,
	}
	svc := newServiceWithStore(store)

	_, err := svc.CreateComputerConfiguration(context.Background(), CreateComputerConfigurationRequest{
		ComputerAssetMasterID: 10,
		PartAssetMasterID:     20,
		PartTypeID:            3,
		InstalledAt:           strPtr("2026-06-10"),
		RemovedAt:             strPtr("2026-06-09"),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != CodeInvalidArgument {
		t.Fatalf("expected INVALID_ARGUMENT, got %v", err)
	}
	if store.createConfigurationCalled {
		t.Fatal("create should not be called on invalid dates")
	}
}

func TestCreateComputerConfigurationRejectsActivePartConflict(t *testing.T) {
	store := &fakeComputerStore{
		assetExists:      true,
		partTypeExists:   true,
		activePartExists: true,
	}
	svc := newServiceWithStore(store)

	_, err := svc.CreateComputerConfiguration(context.Background(), CreateComputerConfigurationRequest{
		ComputerAssetMasterID: 10,
		PartAssetMasterID:     20,
		PartTypeID:            3,
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}

	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != CodeConflict {
		t.Fatalf("expected CONFLICT, got %v", err)
	}
	if store.createConfigurationCalled {
		t.Fatal("create should not be called when active conflict exists")
	}
}

func TestUpdateComputerDetailNormalizesTrimAndClear(t *testing.T) {
	store := &fakeComputerStore{
		updateDetailResponse: &ComputerDetailResponse{AssetMasterID: 7},
	}
	svc := newServiceWithStore(store)

	_, err := svc.UpdateComputerDetail(context.Background(), 7, UpdateComputerDetailRequest{
		Hostname:  strPtr("  host-01  "),
		IPAddress: strPtr(""),
	})
	if err != nil {
		t.Fatalf("UpdateComputerDetail returned error: %v", err)
	}

	if !store.lastUpdateDetailPatch.Hostname.Set || store.lastUpdateDetailPatch.Hostname.Value == nil || *store.lastUpdateDetailPatch.Hostname.Value != "host-01" {
		t.Fatalf("expected trimmed hostname patch, got %#v", store.lastUpdateDetailPatch.Hostname)
	}
	if !store.lastUpdateDetailPatch.IPAddress.Set || store.lastUpdateDetailPatch.IPAddress.Value != nil {
		t.Fatalf("expected ip_address clear patch, got %#v", store.lastUpdateDetailPatch.IPAddress)
	}
}

func TestUpdateComputerConfigurationClearingRemovedAtChecksActiveConflicts(t *testing.T) {
	removedAt := time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC)
	store := &fakeComputerStore{
		getConfigurationResponse: &ComputerConfigurationResponse{
			ComputerConfigurationID: 100,
			ComputerAssetMasterID:   10,
			PartAssetMasterID:       20,
			PartTypeID:              3,
			RemovedAt:               &removedAt,
		},
		activeComputerPartTypeExists: true,
	}
	svc := newServiceWithStore(store)

	_, err := svc.UpdateComputerConfiguration(context.Background(), 100, UpdateComputerConfigurationRequest{
		RemovedAt: strPtr(""),
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}

	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != CodeConflict {
		t.Fatalf("expected CONFLICT, got %v", err)
	}
	if store.updateConfigurationCalled {
		t.Fatal("update should not be called when re-activating causes conflict")
	}
}

func TestListComputerConfigurationsReturnsNotFoundWhenComputerMissing(t *testing.T) {
	store := &fakeComputerStore{}
	svc := newServiceWithStore(store)

	_, err := svc.ListComputerConfigurations(context.Background(), 999)
	if err == nil {
		t.Fatal("expected not found error")
	}
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != CodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
}

func TestParseOptionalDateInputNormalizesUTC(t *testing.T) {
	got, err := parseOptionalDateInput("installed_at", strPtr("2026-06-27"))
	if err != nil {
		t.Fatalf("parseOptionalDateInput returned error: %v", err)
	}
	if got == nil {
		t.Fatal("expected date")
	}
	if !got.Equal(time.Date(2026, time.June, 27, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected date: %v", got)
	}
}

type fakeComputerStore struct {
	assetExists                  bool
	usageStatusExists            bool
	partTypeExists               bool
	activePartExists             bool
	activeComputerPartTypeExists bool

	createConfigurationCalled bool
	updateConfigurationCalled bool

	getConfigurationResponse *ComputerConfigurationResponse
	updateDetailResponse     *ComputerDetailResponse

	lastUpdateDetailPatch updateComputerDetailInput
}

func (f *fakeComputerStore) AssetMasterExists(context.Context, uint64) (bool, error) {
	return f.assetExists, nil
}

func (f *fakeComputerStore) UsageStatusExists(context.Context, uint) (bool, error) {
	return f.usageStatusExists, nil
}

func (f *fakeComputerStore) PartTypeExists(context.Context, uint) (bool, error) {
	return f.partTypeExists, nil
}

func (f *fakeComputerStore) CreateComputerDetail(context.Context, createComputerDetailInput) (*ComputerDetailResponse, error) {
	return nil, nil
}

func (f *fakeComputerStore) GetComputerDetailByAssetMasterID(context.Context, uint64) (*ComputerDetailResponse, error) {
	return nil, sql.ErrNoRows
}

func (f *fakeComputerStore) UpdateComputerDetailByAssetMasterID(_ context.Context, _ uint64, patch updateComputerDetailInput) (*ComputerDetailResponse, error) {
	f.lastUpdateDetailPatch = patch
	if f.updateDetailResponse == nil {
		return nil, sql.ErrNoRows
	}
	return f.updateDetailResponse, nil
}

func (f *fakeComputerStore) CreateComputerPart(context.Context, createComputerPartInput) (*ComputerPartResponse, error) {
	return nil, nil
}

func (f *fakeComputerStore) GetComputerPartByAssetMasterID(context.Context, uint64) (*ComputerPartResponse, error) {
	return nil, sql.ErrNoRows
}

func (f *fakeComputerStore) UpdateComputerPartByAssetMasterID(context.Context, uint64, updateComputerPartInput) (*ComputerPartResponse, error) {
	return nil, sql.ErrNoRows
}

func (f *fakeComputerStore) CreateComputerConfiguration(context.Context, createComputerConfigurationInput) (*ComputerConfigurationResponse, error) {
	f.createConfigurationCalled = true
	return &ComputerConfigurationResponse{}, nil
}

func (f *fakeComputerStore) GetComputerConfigurationByID(context.Context, uint64) (*ComputerConfigurationResponse, error) {
	if f.getConfigurationResponse == nil {
		return nil, sql.ErrNoRows
	}
	return f.getConfigurationResponse, nil
}

func (f *fakeComputerStore) ListComputerConfigurationsByComputerAssetMasterID(context.Context, uint64) ([]ComputerConfigurationResponse, error) {
	return []ComputerConfigurationResponse{}, nil
}

func (f *fakeComputerStore) UpdateComputerConfigurationByID(context.Context, uint64, updateComputerConfigurationInput) (*ComputerConfigurationResponse, error) {
	f.updateConfigurationCalled = true
	return &ComputerConfigurationResponse{}, nil
}

func (f *fakeComputerStore) ActiveConfigurationExistsForPart(context.Context, uint64, *uint64) (bool, error) {
	return f.activePartExists, nil
}

func (f *fakeComputerStore) ActiveConfigurationExistsForComputerPartType(context.Context, uint64, uint, *uint64) (bool, error) {
	return f.activeComputerPartTypeExists, nil
}

func (f *fakeComputerStore) ListPartTypes(context.Context) ([]PartTypeResponse, error) {
	return []PartTypeResponse{}, nil
}

func (f *fakeComputerStore) ListUsageStatuses(context.Context) ([]UsageStatusResponse, error) {
	return []UsageStatusResponse{}, nil
}

func strPtr(v string) *string {
	return &v
}
