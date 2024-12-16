package inventoryService

import (
	"time"

	"github.com/google/uuid"
)

func (InventoryTransaction) TableName() string {
	return "inventory_transaction"
}

func (InventoryTransactionSerial) TableName() string {
	return "inventory_transaction_serial"
}

func (Inventory) TableName() string {
	return "inventory"
}

func (InventorySerial) TableName() string {
	return "inventory_serial"
}

type InventoryTransaction struct {
	ID              uuid.UUID  `json:"id"`
	TenantID        uuid.UUID  `json:"tenant_id"`
	TransactionType string     `json:"transaction_type"`
	CompanyCode     string     `json:"company_code"`
	SiteCode        string     `json:"site_code"`
	WarehouseCode   string     `json:"warehouse_code"`
	ZoneCode        string     `json:"zone_code"`
	LocationCode    string     `json:"location_code"`
	PalletCode      string     `json:"pallet_code"`
	ContainerCode   string     `json:"container_code"`
	StorageType     string     `json:"storage_type"`
	DocumentRefType string     `json:"document_ref_type"`
	DocumentRef     string     `json:"document_ref"`
	ItemRef         string     `json:"item_ref"`
	ProductCode     string     `json:"product_code"`
	ProductType     string     `json:"product_type"`
	Qty             float64    `json:"qty"`
	UnitCode        string     `json:"unit_code"`
	BatchNo         string     `json:"batch_no"`
	Action          string     `json:"action"`
	Status          string     `json:"status"`
	MfgDate         *time.Time `json:"mfg_date"`
	ExpDate         *time.Time `json:"exp_date"`
	CreateBy        string     `json:"create_by"`
	CreateDtm       time.Time  `json:"create_dtm"`
	UpdateBy        string     `json:"update_by"`
	UpdateDtm       time.Time  `json:"update_dtm"`
}

type InventoryTransactionSerial struct {
	ID            uuid.UUID  `json:"id"`
	TransactionID uuid.UUID  `json:"transaction_id"`
	SerialCode    string     `json:"serial_code"`
	Qty           float64    `json:"qty"`
	UnitCode      string     `json:"unit_code"`
	MfgDate       *time.Time `json:"mfg_date"`
	ExpDate       *time.Time `json:"exp_date"`
}

type Inventory struct {
	ID            uuid.UUID         `json:"id"`
	TenantID      uuid.UUID         `json:"tenant_id"`
	StorageType   string            `json:"storage_type"`
	CompanyCode   string            `json:"company_code"`
	SiteCode      string            `json:"site_code"`
	WarehouseCode string            `json:"warehouse_code"`
	ZoneCode      string            `json:"zone_code"`
	LocationCode  string            `json:"location_code"`
	PalletCode    string            `json:"pallet_code"`
	ContainerCode string            `json:"container_code"`
	BatchNo       string            `json:"batch_no"`
	ProductCode   string            `json:"product_code"`
	Qty           float64           `json:"qty"`
	UnitCode      string            `json:"unit_code"`
	MfgDate       *time.Time        `json:"mfg_date"`
	ExpDate       *time.Time        `json:"exp_date"`
	CreateBy      string            `json:"create_by"`
	CreateDtm     time.Time         `json:"create_dtm"`
	UpdateBy      string            `json:"update_by"`
	UpdateDtm     time.Time         `json:"update_dtm"`
	Serials       []InventorySerial `gorm:"foreignKey:InventoryID" json:"serials"`
}

type InventorySerial struct {
	ID          uuid.UUID  `json:"id"`
	InventoryID uuid.UUID  `json:"inventory_id"`
	SerialCode  string     `json:"serial_code"`
	Qty         float64    `json:"qty"`
	UnitCode    string     `json:"unit_code"`
	MfgDate     *time.Time `json:"mfg_date"`
	ExpDate     *time.Time `json:"exp_date"`
	CreateBy    string     `json:"create_by"`
	CreateDtm   time.Time  `json:"create_dtm"`
	UpdateBy    string     `json:"update_by"`
	UpdateDtm   time.Time  `json:"update_dtm"`
}
