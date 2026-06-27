package computers

import "time"

type nullableStringField struct {
	Set   bool
	Value *string
}

type nullableTimeField struct {
	Set   bool
	Value *time.Time
}

type createComputerDetailInput struct {
	AssetMasterID uint64
	Hostname      *string
	IPAddress     *string
	MACAddress    *string
	OS            *string
	Purpose       *string
	LoginUser     *string
	Note          *string
}

type updateComputerDetailInput struct {
	Hostname   nullableStringField
	IPAddress  nullableStringField
	MACAddress nullableStringField
	OS         nullableStringField
	Purpose    nullableStringField
	LoginUser  nullableStringField
	Note       nullableStringField
}

type createComputerPartInput struct {
	AssetMasterID uint64
	UsageStatusID uint
	Specification *string
	Note          *string
}

type updateComputerPartInput struct {
	UsageStatusID *uint
	Specification nullableStringField
	Note          nullableStringField
}

type createComputerConfigurationInput struct {
	ComputerAssetMasterID uint64
	PartAssetMasterID     uint64
	PartTypeID            uint
	InstalledAt           *time.Time
	RemovedAt             *time.Time
	Note                  *string
}

type updateComputerConfigurationInput struct {
	PartAssetMasterID *uint64
	PartTypeID        *uint
	InstalledAt       nullableTimeField
	RemovedAt         nullableTimeField
	Note              nullableStringField
}

type resolvedComputerConfiguration struct {
	ComputerAssetMasterID uint64
	PartAssetMasterID     uint64
	PartTypeID            uint
	InstalledAt           *time.Time
	RemovedAt             *time.Time
}
