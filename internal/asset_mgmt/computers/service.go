package computers

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

type computerStore interface {
	AssetMasterExists(ctx context.Context, assetMasterID uint64) (bool, error)
	UsageStatusExists(ctx context.Context, usageStatusID uint) (bool, error)
	PartTypeExists(ctx context.Context, partTypeID uint) (bool, error)

	CreateComputerDetail(ctx context.Context, in createComputerDetailInput) (*ComputerDetailResponse, error)
	GetComputerDetailByAssetMasterID(ctx context.Context, assetMasterID uint64) (*ComputerDetailResponse, error)
	UpdateComputerDetailByAssetMasterID(ctx context.Context, assetMasterID uint64, patch updateComputerDetailInput) (*ComputerDetailResponse, error)

	CreateComputerPart(ctx context.Context, in createComputerPartInput) (*ComputerPartResponse, error)
	GetComputerPartByAssetMasterID(ctx context.Context, assetMasterID uint64) (*ComputerPartResponse, error)
	UpdateComputerPartByAssetMasterID(ctx context.Context, assetMasterID uint64, patch updateComputerPartInput) (*ComputerPartResponse, error)

	CreateComputerConfiguration(ctx context.Context, in createComputerConfigurationInput) (*ComputerConfigurationResponse, error)
	GetComputerConfigurationByID(ctx context.Context, computerConfigurationID uint64) (*ComputerConfigurationResponse, error)
	ListComputerConfigurationsByComputerAssetMasterID(ctx context.Context, computerAssetMasterID uint64) ([]ComputerConfigurationResponse, error)
	UpdateComputerConfigurationByID(ctx context.Context, computerConfigurationID uint64, patch updateComputerConfigurationInput) (*ComputerConfigurationResponse, error)
	ActiveConfigurationExistsForPart(ctx context.Context, partAssetMasterID uint64, excludeID *uint64) (bool, error)
	ActiveConfigurationExistsForComputerPartType(ctx context.Context, computerAssetMasterID uint64, partTypeID uint, excludeID *uint64) (bool, error)

	ListPartTypes(ctx context.Context) ([]PartTypeResponse, error)
	ListUsageStatuses(ctx context.Context) ([]UsageStatusResponse, error)
}

type Service struct {
	store computerStore
}

func NewService(db *sql.DB) *Service {
	return &Service{store: NewStore(db)}
}

func newServiceWithStore(store computerStore) *Service {
	return &Service{store: store}
}

func (s *Service) CreateComputerDetail(ctx context.Context, req CreateComputerDetailRequest) (ComputerDetailResponse, error) {
	if req.AssetMasterID == 0 {
		return ComputerDetailResponse{}, ErrInvalid("asset_master_id is required")
	}
	if err := s.requireAssetMaster(ctx, req.AssetMasterID, "asset_master_id not found"); err != nil {
		return ComputerDetailResponse{}, err
	}

	out, err := s.store.CreateComputerDetail(ctx, createComputerDetailInput{
		AssetMasterID: req.AssetMasterID,
		Hostname:      normalizeOptionalString(req.Hostname),
		IPAddress:     normalizeOptionalString(req.IPAddress),
		MACAddress:    normalizeOptionalString(req.MACAddress),
		OS:            normalizeOptionalString(req.OS),
		Purpose:       normalizeOptionalString(req.Purpose),
		LoginUser:     normalizeOptionalString(req.LoginUser),
		Note:          normalizeOptionalString(req.Note),
	})
	if err != nil {
		return ComputerDetailResponse{}, mapCreateMySQLError(err, "computer detail already exists", "invalid asset_master_id")
	}
	return *out, nil
}

func (s *Service) GetComputerDetail(ctx context.Context, assetMasterID uint64) (ComputerDetailResponse, error) {
	out, err := s.store.GetComputerDetailByAssetMasterID(ctx, assetMasterID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ComputerDetailResponse{}, ErrNotFound("computer detail not found")
		}
		return ComputerDetailResponse{}, err
	}
	return *out, nil
}

func (s *Service) UpdateComputerDetail(ctx context.Context, assetMasterID uint64, req UpdateComputerDetailRequest) (ComputerDetailResponse, error) {
	out, err := s.store.UpdateComputerDetailByAssetMasterID(ctx, assetMasterID, updateComputerDetailInput{
		Hostname:   normalizeNullableStringField(req.Hostname),
		IPAddress:  normalizeNullableStringField(req.IPAddress),
		MACAddress: normalizeNullableStringField(req.MACAddress),
		OS:         normalizeNullableStringField(req.OS),
		Purpose:    normalizeNullableStringField(req.Purpose),
		LoginUser:  normalizeNullableStringField(req.LoginUser),
		Note:       normalizeNullableStringField(req.Note),
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return ComputerDetailResponse{}, ErrNotFound("computer detail not found")
		}
		return ComputerDetailResponse{}, err
	}
	return *out, nil
}

func (s *Service) CreateComputerPart(ctx context.Context, req CreateComputerPartRequest) (ComputerPartResponse, error) {
	if req.AssetMasterID == 0 {
		return ComputerPartResponse{}, ErrInvalid("asset_master_id is required")
	}
	if req.UsageStatusID == 0 {
		return ComputerPartResponse{}, ErrInvalid("usage_status_id is required")
	}
	if err := s.requireAssetMaster(ctx, req.AssetMasterID, "asset_master_id not found"); err != nil {
		return ComputerPartResponse{}, err
	}
	if err := s.requireUsageStatus(ctx, req.UsageStatusID); err != nil {
		return ComputerPartResponse{}, err
	}

	out, err := s.store.CreateComputerPart(ctx, createComputerPartInput{
		AssetMasterID: req.AssetMasterID,
		UsageStatusID: req.UsageStatusID,
		Specification: normalizeOptionalString(req.Specification),
		Note:          normalizeOptionalString(req.Note),
	})
	if err != nil {
		return ComputerPartResponse{}, mapCreateMySQLError(err, "computer part already exists", "invalid asset_master_id or usage_status_id")
	}
	return *out, nil
}

func (s *Service) GetComputerPart(ctx context.Context, assetMasterID uint64) (ComputerPartResponse, error) {
	out, err := s.store.GetComputerPartByAssetMasterID(ctx, assetMasterID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ComputerPartResponse{}, ErrNotFound("computer part not found")
		}
		return ComputerPartResponse{}, err
	}
	return *out, nil
}

func (s *Service) UpdateComputerPart(ctx context.Context, assetMasterID uint64, req UpdateComputerPartRequest) (ComputerPartResponse, error) {
	if req.UsageStatusID != nil {
		if *req.UsageStatusID == 0 {
			return ComputerPartResponse{}, ErrInvalid("usage_status_id must be greater than 0")
		}
		if err := s.requireUsageStatus(ctx, *req.UsageStatusID); err != nil {
			return ComputerPartResponse{}, err
		}
	}

	out, err := s.store.UpdateComputerPartByAssetMasterID(ctx, assetMasterID, updateComputerPartInput{
		UsageStatusID: req.UsageStatusID,
		Specification: normalizeNullableStringField(req.Specification),
		Note:          normalizeNullableStringField(req.Note),
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return ComputerPartResponse{}, ErrNotFound("computer part not found")
		}
		return ComputerPartResponse{}, err
	}
	return *out, nil
}

func (s *Service) CreateComputerConfiguration(ctx context.Context, req CreateComputerConfigurationRequest) (ComputerConfigurationResponse, error) {
	if req.ComputerAssetMasterID == 0 || req.PartAssetMasterID == 0 || req.PartTypeID == 0 {
		return ComputerConfigurationResponse{}, ErrInvalid("computer_asset_master_id, part_asset_master_id, part_type_id are required")
	}
	if err := s.requireAssetMaster(ctx, req.ComputerAssetMasterID, "computer_asset_master_id not found"); err != nil {
		return ComputerConfigurationResponse{}, err
	}
	if err := s.requireAssetMaster(ctx, req.PartAssetMasterID, "part_asset_master_id not found"); err != nil {
		return ComputerConfigurationResponse{}, err
	}
	if err := s.requirePartType(ctx, req.PartTypeID); err != nil {
		return ComputerConfigurationResponse{}, err
	}

	installedAt, err := parseOptionalDateInput("installed_at", req.InstalledAt)
	if err != nil {
		return ComputerConfigurationResponse{}, err
	}
	removedAt, err := parseOptionalDateInput("removed_at", req.RemovedAt)
	if err != nil {
		return ComputerConfigurationResponse{}, err
	}
	if err := validateConfigurationDates(installedAt, removedAt); err != nil {
		return ComputerConfigurationResponse{}, err
	}

	if removedAt == nil {
		if err := s.ensureActiveConfigurationAvailable(ctx, req.PartAssetMasterID, req.ComputerAssetMasterID, req.PartTypeID, nil); err != nil {
			return ComputerConfigurationResponse{}, err
		}
	}

	out, err := s.store.CreateComputerConfiguration(ctx, createComputerConfigurationInput{
		ComputerAssetMasterID: req.ComputerAssetMasterID,
		PartAssetMasterID:     req.PartAssetMasterID,
		PartTypeID:            req.PartTypeID,
		InstalledAt:           installedAt,
		RemovedAt:             removedAt,
		Note:                  normalizeOptionalString(req.Note),
	})
	if err != nil {
		return ComputerConfigurationResponse{}, mapCreateMySQLError(err, "computer configuration already exists", "invalid computer configuration reference")
	}
	return *out, nil
}

func (s *Service) ListComputerConfigurations(ctx context.Context, computerAssetMasterID uint64) ([]ComputerConfigurationResponse, error) {
	exists, err := s.store.AssetMasterExists(ctx, computerAssetMasterID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound("computer asset master not found")
	}
	return s.store.ListComputerConfigurationsByComputerAssetMasterID(ctx, computerAssetMasterID)
}

func (s *Service) UpdateComputerConfiguration(ctx context.Context, computerConfigurationID uint64, req UpdateComputerConfigurationRequest) (ComputerConfigurationResponse, error) {
	current, err := s.store.GetComputerConfigurationByID(ctx, computerConfigurationID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ComputerConfigurationResponse{}, ErrNotFound("computer configuration not found")
		}
		return ComputerConfigurationResponse{}, err
	}

	installedAt, err := parseNullableDateField("installed_at", req.InstalledAt)
	if err != nil {
		return ComputerConfigurationResponse{}, err
	}
	removedAt, err := parseNullableDateField("removed_at", req.RemovedAt)
	if err != nil {
		return ComputerConfigurationResponse{}, err
	}

	if req.PartAssetMasterID != nil {
		if *req.PartAssetMasterID == 0 {
			return ComputerConfigurationResponse{}, ErrInvalid("part_asset_master_id must be greater than 0")
		}
		if err := s.requireAssetMaster(ctx, *req.PartAssetMasterID, "part_asset_master_id not found"); err != nil {
			return ComputerConfigurationResponse{}, err
		}
	}
	if req.PartTypeID != nil {
		if *req.PartTypeID == 0 {
			return ComputerConfigurationResponse{}, ErrInvalid("part_type_id must be greater than 0")
		}
		if err := s.requirePartType(ctx, *req.PartTypeID); err != nil {
			return ComputerConfigurationResponse{}, err
		}
	}

	resolved := resolvedComputerConfiguration{
		ComputerAssetMasterID: current.ComputerAssetMasterID,
		PartAssetMasterID:     current.PartAssetMasterID,
		PartTypeID:            current.PartTypeID,
		InstalledAt:           current.InstalledAt,
		RemovedAt:             current.RemovedAt,
	}
	if req.PartAssetMasterID != nil {
		resolved.PartAssetMasterID = *req.PartAssetMasterID
	}
	if req.PartTypeID != nil {
		resolved.PartTypeID = *req.PartTypeID
	}
	if installedAt.Set {
		resolved.InstalledAt = installedAt.Value
	}
	if removedAt.Set {
		resolved.RemovedAt = removedAt.Value
	}

	if err := validateConfigurationDates(resolved.InstalledAt, resolved.RemovedAt); err != nil {
		return ComputerConfigurationResponse{}, err
	}
	if resolved.RemovedAt == nil {
		if err := s.ensureActiveConfigurationAvailable(ctx, resolved.PartAssetMasterID, resolved.ComputerAssetMasterID, resolved.PartTypeID, &computerConfigurationID); err != nil {
			return ComputerConfigurationResponse{}, err
		}
	}

	out, err := s.store.UpdateComputerConfigurationByID(ctx, computerConfigurationID, updateComputerConfigurationInput{
		PartAssetMasterID: req.PartAssetMasterID,
		PartTypeID:        req.PartTypeID,
		InstalledAt:       installedAt,
		RemovedAt:         removedAt,
		Note:              normalizeNullableStringField(req.Note),
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return ComputerConfigurationResponse{}, ErrNotFound("computer configuration not found")
		}
		return ComputerConfigurationResponse{}, err
	}
	return *out, nil
}

func (s *Service) ListPartTypes(ctx context.Context) ([]PartTypeResponse, error) {
	return s.store.ListPartTypes(ctx)
}

func (s *Service) ListUsageStatuses(ctx context.Context) ([]UsageStatusResponse, error) {
	return s.store.ListUsageStatuses(ctx)
}

func (s *Service) requireAssetMaster(ctx context.Context, assetMasterID uint64, msg string) error {
	exists, err := s.store.AssetMasterExists(ctx, assetMasterID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrInvalid(msg)
	}
	return nil
}

func (s *Service) requireUsageStatus(ctx context.Context, usageStatusID uint) error {
	exists, err := s.store.UsageStatusExists(ctx, usageStatusID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrInvalid("usage_status_id not found")
	}
	return nil
}

func (s *Service) requirePartType(ctx context.Context, partTypeID uint) error {
	exists, err := s.store.PartTypeExists(ctx, partTypeID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrInvalid("part_type_id not found")
	}
	return nil
}

func (s *Service) ensureActiveConfigurationAvailable(ctx context.Context, partAssetMasterID uint64, computerAssetMasterID uint64, partTypeID uint, excludeID *uint64) error {
	partInUse, err := s.store.ActiveConfigurationExistsForPart(ctx, partAssetMasterID, excludeID)
	if err != nil {
		return err
	}
	if partInUse {
		return ErrConflict("part asset is already assigned to an active computer configuration")
	}

	slotInUse, err := s.store.ActiveConfigurationExistsForComputerPartType(ctx, computerAssetMasterID, partTypeID, excludeID)
	if err != nil {
		return err
	}
	if slotInUse {
		return ErrConflict("computer already has an active configuration for this part_type_id")
	}
	return nil
}

func parseOptionalDateInput(field string, raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return nil, ErrInvalid(field + " must be YYYY-MM-DD")
	}
	t = t.UTC()
	return &t, nil
}

func parseNullableDateField(field string, raw *string) (nullableTimeField, error) {
	if raw == nil {
		return nullableTimeField{}, nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nullableTimeField{Set: true, Value: nil}, nil
	}
	t, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return nullableTimeField{}, ErrInvalid(field + " must be YYYY-MM-DD")
	}
	t = t.UTC()
	return nullableTimeField{Set: true, Value: &t}, nil
}

func validateConfigurationDates(installedAt, removedAt *time.Time) error {
	if installedAt != nil && removedAt != nil && removedAt.Before(*installedAt) {
		return ErrInvalid("removed_at must be on or after installed_at")
	}
	return nil
}

func normalizeOptionalString(raw *string) *string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeNullableStringField(raw *string) nullableStringField {
	if raw == nil {
		return nullableStringField{}
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nullableStringField{Set: true, Value: nil}
	}
	return nullableStringField{Set: true, Value: &trimmed}
}

func mapCreateMySQLError(err error, duplicateMsg, foreignKeyMsg string) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		switch me.Number {
		case 1062:
			return ErrConflict(duplicateMsg)
		case 1452:
			return ErrInvalid(foreignKeyMsg)
		}
	}
	return err
}
