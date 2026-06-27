package computers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) AssetMasterExists(ctx context.Context, assetMasterID uint64) (bool, error) {
	return s.exists(ctx, "SELECT 1 FROM assets_master WHERE asset_master_id = ?", assetMasterID)
}

func (s *Store) UsageStatusExists(ctx context.Context, usageStatusID uint) (bool, error) {
	return s.exists(ctx, "SELECT 1 FROM usage_status WHERE usage_status_id = ?", usageStatusID)
}

func (s *Store) PartTypeExists(ctx context.Context, partTypeID uint) (bool, error) {
	return s.exists(ctx, "SELECT 1 FROM part_types WHERE part_type_id = ?", partTypeID)
}

func (s *Store) CreateComputerDetail(ctx context.Context, in createComputerDetailInput) (*ComputerDetailResponse, error) {
	const q = `
	INSERT INTO computer_details
		(asset_master_id, hostname, ip_address, mac_address, os, purpose, login_user, note)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	if _, err := s.db.ExecContext(ctx, q,
		in.AssetMasterID,
		in.Hostname,
		in.IPAddress,
		in.MACAddress,
		in.OS,
		in.Purpose,
		in.LoginUser,
		in.Note,
	); err != nil {
		return nil, err
	}

	return s.GetComputerDetailByAssetMasterID(ctx, in.AssetMasterID)
}

func (s *Store) GetComputerDetailByAssetMasterID(ctx context.Context, assetMasterID uint64) (*ComputerDetailResponse, error) {
	const q = `
	SELECT
		cd.computer_detail_id,
		cd.asset_master_id,
		am.management_number,
		am.name,
		cd.hostname,
		cd.ip_address,
		cd.mac_address,
		cd.os,
		cd.purpose,
		cd.login_user,
		cd.note,
		cd.created_at,
		cd.updated_at
	FROM computer_details cd
	JOIN assets_master am ON am.asset_master_id = cd.asset_master_id
	WHERE cd.asset_master_id = ?`

	var out ComputerDetailResponse
	var hostname, ipAddress, macAddress, osValue, purpose, loginUser, note sql.NullString
	if err := s.db.QueryRowContext(ctx, q, assetMasterID).Scan(
		&out.ComputerDetailID,
		&out.AssetMasterID,
		&out.ManagementNumber,
		&out.AssetName,
		&hostname,
		&ipAddress,
		&macAddress,
		&osValue,
		&purpose,
		&loginUser,
		&note,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}

	out.Hostname = ptrString(hostname)
	out.IPAddress = ptrString(ipAddress)
	out.MACAddress = ptrString(macAddress)
	out.OS = ptrString(osValue)
	out.Purpose = ptrString(purpose)
	out.LoginUser = ptrString(loginUser)
	out.Note = ptrString(note)

	return &out, nil
}

func (s *Store) UpdateComputerDetailByAssetMasterID(ctx context.Context, assetMasterID uint64, patch updateComputerDetailInput) (*ComputerDetailResponse, error) {
	sets := make([]string, 0, 7)
	args := make([]any, 0, 7)

	appendNullableStringUpdate("hostname", patch.Hostname, &sets, &args)
	appendNullableStringUpdate("ip_address", patch.IPAddress, &sets, &args)
	appendNullableStringUpdate("mac_address", patch.MACAddress, &sets, &args)
	appendNullableStringUpdate("os", patch.OS, &sets, &args)
	appendNullableStringUpdate("purpose", patch.Purpose, &sets, &args)
	appendNullableStringUpdate("login_user", patch.LoginUser, &sets, &args)
	appendNullableStringUpdate("note", patch.Note, &sets, &args)

	if len(sets) == 0 {
		return s.GetComputerDetailByAssetMasterID(ctx, assetMasterID)
	}

	args = append(args, assetMasterID)
	q := fmt.Sprintf("UPDATE computer_details SET %s WHERE asset_master_id = ?", strings.Join(sets, ", "))
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	if aff, _ := res.RowsAffected(); aff == 0 {
		if _, err := s.GetComputerDetailByAssetMasterID(ctx, assetMasterID); err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
	}

	return s.GetComputerDetailByAssetMasterID(ctx, assetMasterID)
}

func (s *Store) CreateComputerPart(ctx context.Context, in createComputerPartInput) (*ComputerPartResponse, error) {
	const q = `
	INSERT INTO computer_parts
		(asset_master_id, usage_status_id, spec, note)
	VALUES (?, ?, ?, ?)`

	if _, err := s.db.ExecContext(ctx, q,
		in.AssetMasterID,
		in.UsageStatusID,
		in.Specification,
		in.Note,
	); err != nil {
		return nil, err
	}

	return s.GetComputerPartByAssetMasterID(ctx, in.AssetMasterID)
}

func (s *Store) GetComputerPartByAssetMasterID(ctx context.Context, assetMasterID uint64) (*ComputerPartResponse, error) {
	const q = `
	SELECT
		cp.computer_part_id,
		cp.asset_master_id,
		am.management_number,
		am.name,
		cp.usage_status_id,
		us.name,
		us.display_name,
		cp.spec,
		cp.note,
		cp.created_at,
		cp.updated_at
	FROM computer_parts cp
	JOIN assets_master am ON am.asset_master_id = cp.asset_master_id
	JOIN usage_status us ON us.usage_status_id = cp.usage_status_id
	WHERE cp.asset_master_id = ?`

	var out ComputerPartResponse
	var spec, note sql.NullString
	if err := s.db.QueryRowContext(ctx, q, assetMasterID).Scan(
		&out.ComputerPartID,
		&out.AssetMasterID,
		&out.ManagementNumber,
		&out.AssetName,
		&out.UsageStatusID,
		&out.UsageStatusName,
		&out.UsageStatusDisplayName,
		&spec,
		&note,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}

	out.Specification = ptrString(spec)
	out.Note = ptrString(note)
	return &out, nil
}

func (s *Store) UpdateComputerPartByAssetMasterID(ctx context.Context, assetMasterID uint64, patch updateComputerPartInput) (*ComputerPartResponse, error) {
	sets := make([]string, 0, 3)
	args := make([]any, 0, 3)

	if patch.UsageStatusID != nil {
		sets = append(sets, "usage_status_id = ?")
		args = append(args, *patch.UsageStatusID)
	}
	appendNullableStringUpdate("spec", patch.Specification, &sets, &args)
	appendNullableStringUpdate("note", patch.Note, &sets, &args)

	if len(sets) == 0 {
		return s.GetComputerPartByAssetMasterID(ctx, assetMasterID)
	}

	args = append(args, assetMasterID)
	q := fmt.Sprintf("UPDATE computer_parts SET %s WHERE asset_master_id = ?", strings.Join(sets, ", "))
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	if aff, _ := res.RowsAffected(); aff == 0 {
		if _, err := s.GetComputerPartByAssetMasterID(ctx, assetMasterID); err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
	}

	return s.GetComputerPartByAssetMasterID(ctx, assetMasterID)
}

func (s *Store) CreateComputerConfiguration(ctx context.Context, in createComputerConfigurationInput) (*ComputerConfigurationResponse, error) {
	const q = `
	INSERT INTO computer_configurations
		(computer_asset_master_id, part_asset_master_id, part_type_id, installed_at, removed_at, note)
	VALUES (?, ?, ?, ?, ?, ?)`

	res, err := s.db.ExecContext(ctx, q,
		in.ComputerAssetMasterID,
		in.PartAssetMasterID,
		in.PartTypeID,
		in.InstalledAt,
		in.RemovedAt,
		in.Note,
	)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return s.GetComputerConfigurationByID(ctx, uint64(id))
}

func (s *Store) GetComputerConfigurationByID(ctx context.Context, computerConfigurationID uint64) (*ComputerConfigurationResponse, error) {
	const q = `
	SELECT
		c.computer_configuration_id,
		c.computer_asset_master_id,
		cm.management_number,
		cm.name,
		c.part_asset_master_id,
		pm.management_number,
		pm.name,
		c.part_type_id,
		pt.name,
		pt.display_name,
		c.installed_at,
		c.removed_at,
		c.note,
		c.created_at,
		c.updated_at
	FROM computer_configurations c
	JOIN assets_master cm ON cm.asset_master_id = c.computer_asset_master_id
	JOIN assets_master pm ON pm.asset_master_id = c.part_asset_master_id
	JOIN part_types pt ON pt.part_type_id = c.part_type_id
	WHERE c.computer_configuration_id = ?`

	return s.queryComputerConfiguration(ctx, q, computerConfigurationID)
}

func (s *Store) ListComputerConfigurationsByComputerAssetMasterID(ctx context.Context, computerAssetMasterID uint64) ([]ComputerConfigurationResponse, error) {
	const q = `
	SELECT
		c.computer_configuration_id,
		c.computer_asset_master_id,
		cm.management_number,
		cm.name,
		c.part_asset_master_id,
		pm.management_number,
		pm.name,
		c.part_type_id,
		pt.name,
		pt.display_name,
		c.installed_at,
		c.removed_at,
		c.note,
		c.created_at,
		c.updated_at
	FROM computer_configurations c
	JOIN assets_master cm ON cm.asset_master_id = c.computer_asset_master_id
	JOIN assets_master pm ON pm.asset_master_id = c.part_asset_master_id
	JOIN part_types pt ON pt.part_type_id = c.part_type_id
	WHERE c.computer_asset_master_id = ?
	ORDER BY c.removed_at IS NULL DESC, c.part_type_id ASC, c.computer_configuration_id DESC`

	rows, err := s.db.QueryContext(ctx, q, computerAssetMasterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ComputerConfigurationResponse, 0, 8)
	for rows.Next() {
		item, err := scanComputerConfiguration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) UpdateComputerConfigurationByID(ctx context.Context, computerConfigurationID uint64, patch updateComputerConfigurationInput) (*ComputerConfigurationResponse, error) {
	sets := make([]string, 0, 5)
	args := make([]any, 0, 5)

	if patch.PartAssetMasterID != nil {
		sets = append(sets, "part_asset_master_id = ?")
		args = append(args, *patch.PartAssetMasterID)
	}
	if patch.PartTypeID != nil {
		sets = append(sets, "part_type_id = ?")
		args = append(args, *patch.PartTypeID)
	}
	appendNullableTimeUpdate("installed_at", patch.InstalledAt, &sets, &args)
	appendNullableTimeUpdate("removed_at", patch.RemovedAt, &sets, &args)
	appendNullableStringUpdate("note", patch.Note, &sets, &args)

	if len(sets) == 0 {
		return s.GetComputerConfigurationByID(ctx, computerConfigurationID)
	}

	args = append(args, computerConfigurationID)
	q := fmt.Sprintf("UPDATE computer_configurations SET %s WHERE computer_configuration_id = ?", strings.Join(sets, ", "))
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	if aff, _ := res.RowsAffected(); aff == 0 {
		if _, err := s.GetComputerConfigurationByID(ctx, computerConfigurationID); err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
	}

	return s.GetComputerConfigurationByID(ctx, computerConfigurationID)
}

func (s *Store) ActiveConfigurationExistsForPart(ctx context.Context, partAssetMasterID uint64, excludeID *uint64) (bool, error) {
	query := "SELECT 1 FROM computer_configurations WHERE part_asset_master_id = ? AND removed_at IS NULL"
	args := []any{partAssetMasterID}
	if excludeID != nil {
		query += " AND computer_configuration_id <> ?"
		args = append(args, *excludeID)
	}
	query += " LIMIT 1"
	return s.exists(ctx, query, args...)
}

func (s *Store) ActiveConfigurationExistsForComputerPartType(ctx context.Context, computerAssetMasterID uint64, partTypeID uint, excludeID *uint64) (bool, error) {
	query := "SELECT 1 FROM computer_configurations WHERE computer_asset_master_id = ? AND part_type_id = ? AND removed_at IS NULL"
	args := []any{computerAssetMasterID, partTypeID}
	if excludeID != nil {
		query += " AND computer_configuration_id <> ?"
		args = append(args, *excludeID)
	}
	query += " LIMIT 1"
	return s.exists(ctx, query, args...)
}

func (s *Store) ListPartTypes(ctx context.Context) ([]PartTypeResponse, error) {
	const q = `
	SELECT part_type_id, name, display_name, note
	FROM part_types
	ORDER BY part_type_id`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]PartTypeResponse, 0, 8)
	for rows.Next() {
		var item PartTypeResponse
		var note sql.NullString
		if err := rows.Scan(&item.PartTypeID, &item.Name, &item.DisplayName, &note); err != nil {
			return nil, err
		}
		item.Note = ptrString(note)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListUsageStatuses(ctx context.Context) ([]UsageStatusResponse, error) {
	const q = `
	SELECT usage_status_id, name, display_name, note
	FROM usage_status
	ORDER BY usage_status_id`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UsageStatusResponse, 0, 8)
	for rows.Next() {
		var item UsageStatusResponse
		var note sql.NullString
		if err := rows.Scan(&item.UsageStatusID, &item.Name, &item.DisplayName, &note); err != nil {
			return nil, err
		}
		item.Note = ptrString(note)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) exists(ctx context.Context, query string, args ...any) (bool, error) {
	var dummy int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&dummy)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) queryComputerConfiguration(ctx context.Context, query string, args ...any) (*ComputerConfigurationResponse, error) {
	row := s.db.QueryRowContext(ctx, query, args...)
	item, err := scanComputerConfiguration(row)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanComputerConfiguration(s scanner) (ComputerConfigurationResponse, error) {
	var out ComputerConfigurationResponse
	var installedAt, removedAt sql.NullTime
	var note sql.NullString
	err := s.Scan(
		&out.ComputerConfigurationID,
		&out.ComputerAssetMasterID,
		&out.ComputerManagementNumber,
		&out.ComputerName,
		&out.PartAssetMasterID,
		&out.PartManagementNumber,
		&out.PartName,
		&out.PartTypeID,
		&out.PartTypeName,
		&out.PartTypeDisplayName,
		&installedAt,
		&removedAt,
		&note,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return ComputerConfigurationResponse{}, err
	}

	out.InstalledAt = ptrTime(installedAt)
	out.RemovedAt = ptrTime(removedAt)
	out.Note = ptrString(note)
	return out, nil
}

func appendNullableStringUpdate(column string, field nullableStringField, sets *[]string, args *[]any) {
	if !field.Set {
		return
	}
	if field.Value == nil {
		*sets = append(*sets, column+" = NULL")
		return
	}
	*sets = append(*sets, column+" = ?")
	*args = append(*args, *field.Value)
}

func appendNullableTimeUpdate(column string, field nullableTimeField, sets *[]string, args *[]any) {
	if !field.Set {
		return
	}
	if field.Value == nil {
		*sets = append(*sets, column+" = NULL")
		return
	}
	*sets = append(*sets, column+" = ?")
	*args = append(*args, *field.Value)
}

func ptrString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

func ptrTime(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	v := nt.Time
	return &v
}
