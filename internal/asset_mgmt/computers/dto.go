package computers

import "time"

type CreateComputerDetailRequest struct {
	AssetMasterID uint64  `json:"asset_master_id" binding:"required"`
	Hostname      *string `json:"hostname,omitempty"`
	IPAddress     *string `json:"ip_address,omitempty"`
	MACAddress    *string `json:"mac_address,omitempty"`
	OS            *string `json:"os,omitempty"`
	Purpose       *string `json:"purpose,omitempty"`
	LoginUser     *string `json:"login_user,omitempty"`
	Note          *string `json:"note,omitempty"`
}

type UpdateComputerDetailRequest struct {
	Hostname   *string `json:"hostname,omitempty"`
	IPAddress  *string `json:"ip_address,omitempty"`
	MACAddress *string `json:"mac_address,omitempty"`
	OS         *string `json:"os,omitempty"`
	Purpose    *string `json:"purpose,omitempty"`
	LoginUser  *string `json:"login_user,omitempty"`
	Note       *string `json:"note,omitempty"`
}

type CreateComputerPartRequest struct {
	AssetMasterID uint64  `json:"asset_master_id" binding:"required"`
	UsageStatusID uint    `json:"usage_status_id" binding:"required"`
	Specification *string `json:"spec,omitempty"`
	Note          *string `json:"note,omitempty"`
}

type UpdateComputerPartRequest struct {
	UsageStatusID *uint   `json:"usage_status_id,omitempty"`
	Specification *string `json:"spec,omitempty"`
	Note          *string `json:"note,omitempty"`
}

type CreateComputerConfigurationRequest struct {
	ComputerAssetMasterID uint64  `json:"computer_asset_master_id" binding:"required"`
	PartAssetMasterID     uint64  `json:"part_asset_master_id" binding:"required"`
	PartTypeID            uint    `json:"part_type_id" binding:"required"`
	InstalledAt           *string `json:"installed_at,omitempty"`
	RemovedAt             *string `json:"removed_at,omitempty"`
	Note                  *string `json:"note,omitempty"`
}

type UpdateComputerConfigurationRequest struct {
	PartAssetMasterID *uint64 `json:"part_asset_master_id,omitempty"`
	PartTypeID        *uint   `json:"part_type_id,omitempty"`
	InstalledAt       *string `json:"installed_at,omitempty"`
	RemovedAt         *string `json:"removed_at,omitempty"`
	Note              *string `json:"note,omitempty"`
}

type ComputerDetailResponse struct {
	ComputerDetailID uint64    `json:"computer_detail_id"`
	AssetMasterID    uint64    `json:"asset_master_id"`
	ManagementNumber string    `json:"management_number"`
	AssetName        string    `json:"asset_name"`
	Hostname         *string   `json:"hostname,omitempty"`
	IPAddress        *string   `json:"ip_address,omitempty"`
	MACAddress       *string   `json:"mac_address,omitempty"`
	OS               *string   `json:"os,omitempty"`
	Purpose          *string   `json:"purpose,omitempty"`
	LoginUser        *string   `json:"login_user,omitempty"`
	Note             *string   `json:"note,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ComputerPartResponse struct {
	ComputerPartID         uint64    `json:"computer_part_id"`
	AssetMasterID          uint64    `json:"asset_master_id"`
	ManagementNumber       string    `json:"management_number"`
	AssetName              string    `json:"asset_name"`
	UsageStatusID          uint      `json:"usage_status_id"`
	UsageStatusName        string    `json:"usage_status_name"`
	UsageStatusDisplayName string    `json:"usage_status_display_name"`
	Specification          *string   `json:"spec,omitempty"`
	Note                   *string   `json:"note,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type ComputerConfigurationResponse struct {
	ComputerConfigurationID  uint64     `json:"computer_configuration_id"`
	ComputerAssetMasterID    uint64     `json:"computer_asset_master_id"`
	ComputerManagementNumber string     `json:"computer_management_number"`
	ComputerName             string     `json:"computer_name"`
	PartAssetMasterID        uint64     `json:"part_asset_master_id"`
	PartManagementNumber     string     `json:"part_management_number"`
	PartName                 string     `json:"part_name"`
	PartTypeID               uint       `json:"part_type_id"`
	PartTypeName             string     `json:"part_type_name"`
	PartTypeDisplayName      string     `json:"part_type_display_name"`
	InstalledAt              *time.Time `json:"installed_at,omitempty"`
	RemovedAt                *time.Time `json:"removed_at,omitempty"`
	Note                     *string    `json:"note,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

type PartTypeResponse struct {
	PartTypeID  uint    `json:"part_type_id"`
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	Note        *string `json:"note,omitempty"`
}

type UsageStatusResponse struct {
	UsageStatusID uint    `json:"usage_status_id"`
	Name          string  `json:"name"`
	DisplayName   string  `json:"display_name"`
	Note          *string `json:"note,omitempty"`
}

type ErrorDetail struct {
	Code    string `json:"code" example:"INVALID_ARGUMENT"`
	Message string `json:"message" example:"invalid json"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}
