package inventoryService

import (
	"encoding/json"
	"errors"
	"fmt"
	"jnv-jit/internal/db"
	"math"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"gorm.io/gorm"
)

type InvetoryTask struct {
	Action               string                       `json:"action"` //I = Inbound, M = Movement, O = Outbound
	InventoryTransaction []InventoryTransationRequest `json:"inventory_transaction"`
}

type InventoryTransationRequest struct {
	TenantId          uuid.UUID                           `json:"tenant_id"`
	StorageType       string                              `json:"storage_type"`
	DocumentRef       string                              `json:"document_ref"`
	DocumentRefType   string                              `json:"document_ref_type"`
	ItemRef           string                              `json:"item_ref"`
	SrcCompanyCode    string                              `json:"src_company_code"`
	SrcSiteCode       string                              `json:"src_site_code"`
	SrcWarehouseCode  string                              `json:"src_warehouse_code"`
	SrcZoneCode       string                              `json:"src_zone_code"`
	SrcLocationCode   string                              `json:"src_location_code"`
	SrcPalletCode     string                              `json:"src_pallet_code"`
	SrcContainerCode  string                              `json:"src_container_code"`
	DestCompanyCode   string                              `json:"dest_company_code"`
	DestSiteCode      string                              `json:"dest_site_code"`
	DestWarehouseCode string                              `json:"dest_warehouse_code"`
	DestZoneCode      string                              `json:"dest_zone_code"`
	DestLocationCode  string                              `json:"dest_location_code"`
	DestPalletCode    string                              `json:"dest_pallet_code"`
	DestContainerCode string                              `json:"dest_container_code"`
	ProductCode       string                              `json:"product_code"`
	Qty               float64                             `json:"qty"`
	UnitCode          string                              `json:"unit_code"`
	MfgDate           *time.Time                          `json:"mfg_date"`
	ExpDate           *time.Time                          `json:"exp_date"`
	BatchNo           string                              `json:"batch_no"`
	Serials           []InventoryTransactionSerialRequest `json:"serials"`
}

type InventoryTransactionSerialRequest struct {
	SerialNo string     `json:"serial_no"`
	Qty      float64    `json:"qty"`
	UnitCode string     `json:"unit_code"`
	MfgDate  *time.Time `json:"mfg_date"`
	ExpDate  *time.Time `json:"exp_date"`
}

type ProductConverter struct {
	ProductCode  string  `json:"product_code"`
	SrcUnitCode  string  `json:"src_unit_code"`
	DestUnitCode string  `json:"dest_unit_code"`
	ConvertQty   float64 `json:"convert_qty"`
}

type Product struct {
	ProductCode    string `json:"product_code"`
	IsBatch        bool   `json:"is_batch"`
	IsSerial       bool   `json:"is_serial"`
	IsNotPallet    bool   `json:"is_not_pallet"`
	IsNotContainer bool   `json:"is_not_container"`
}

type InventoryWarehouseConfig struct {
	CompanyCode     string `json:"company_code"`
	SiteCode        string `json:"site_code"`
	WarehouseCode   string `json:"warehouse_code"`
	ZoneCode        string `json:"zone_code"`
	IsNegativeStock bool   `json:"is_negative_stock"`
}

func UpdateInventory(ctx *gin.Context, jsonPayload string) (interface{}, error) {
	var req InvetoryTask
	var invTranReq []InventoryTransationRequest

	if err := json.Unmarshal([]byte(jsonPayload), &req); err != nil {
		return nil, errors.New("failed to unmarshal JSON into struct: " + err.Error())
	}

	invTranReq = req.InventoryTransaction
	action := req.Action

	gormx, err := db.ConnectGORM(`prime_wms_warehouse`)
	if err != nil {
		return nil, err
	}
	defer db.CloseGORM(gormx)

	sqlx, err := db.ConnectSqlx(`prime_wms_warehouse`)
	if err != nil {
		return nil, err
	}
	defer sqlx.Close()

	//Todo get by context
	user := `AAA`
	tenantId := uuid.New()

	invTrans, invTranSerial, err := BuildInvTransaction(invTranReq, action, user, tenantId)
	if err != nil {
		return nil, err
	}

	var productConverter []ProductConverter
	productConverterCheck := map[string]bool{}
	var productsStr []string
	productStrCheck := map[string]bool{}
	var invWhConfig []InventoryWarehouseConfig
	invWhConfigCheck := map[string]bool{}

	for _, item := range invTrans {
		convertKey := fmt.Sprintf(`%s|%s`, item.ProductCode, item.UnitCode)
		if _, exist := productConverterCheck[convertKey]; !exist {
			productConverter = append(productConverter, ProductConverter{
				ProductCode: item.ProductCode,
				SrcUnitCode: item.UnitCode,
			})

			productConverterCheck[convertKey] = true
		}

		productKey := item.ProductCode
		if _, exist := productStrCheck[productKey]; !exist {
			productsStr = append(productsStr, item.ProductCode)

			productStrCheck[productKey] = true
		}

		warehouseConfigKey := fmt.Sprintf(`%s|%s|%s|%s`, item.CompanyCode, item.SiteCode, item.WarehouseCode, item.ZoneCode)
		if _, exist := invWhConfigCheck[warehouseConfigKey]; !exist {
			invWhConfig = append(invWhConfig, InventoryWarehouseConfig{
				CompanyCode:   item.CompanyCode,
				SiteCode:      item.SiteCode,
				WarehouseCode: item.WarehouseCode,
				ZoneCode:      item.ZoneCode,
			})

			invWhConfigCheck[warehouseConfigKey] = true
		}
	}

	productConverterMap, err := GetProductConvert(productConverter)
	if err != nil {
		return nil, err
	}

	invTrans, invTranSerial, err = ConvertUnitInventoryTransaction(invTrans, invTranSerial, productConverterMap)
	if err != nil {
		return nil, err
	}

	productsMap, err := GetProduct(productsStr)
	if err != nil {
		return nil, err
	}

	inventoryMap, err := GetInventoryDB(gormx, invTrans)
	if err != nil {
		return nil, err
	}

	invWhConfigMap, err := GetInventoryWarehouseConfig(sqlx, invWhConfig)
	if err != nil {
		return nil, err
	}

	err = ValidateInvTransaction(action, inventoryMap, invTrans, invTranSerial, productsMap, productConverterMap, invWhConfigMap)
	if err != nil {
		return nil, err
	}

	err = CreateInventoryTransaction(gormx, invTrans, invTranSerial)
	if err != nil {
		return nil, err
	}

	createInv, updateInv, createInvSerial, deleteInvSerial, err := BuildInv(inventoryMap, invTrans, invTranSerial, user, tenantId)
	if err != nil {
		return nil, err
	}

	//todo update status inventory
	err = UpdateInv(gormx, createInv, updateInv, createInvSerial, deleteInvSerial, user, tenantId)
	if err != nil {
		return nil, err
	}

	var inventoryIds []uuid.UUID

	for _, item := range createInv {
		inventoryIds = append(inventoryIds, item.ID)
	}

	for _, item := range updateInv {
		inventoryIds = append(inventoryIds, item.ID)
	}

	DeleteZeroInventory(gormx, inventoryIds)

	return nil, nil
}

func GetProductConvert(productConverter []ProductConverter) (map[string]ProductConverter, error) {
	productConverterMap := map[string]ProductConverter{}

	//Todo get from ProductMaster
	productConverter = []ProductConverter{
		{
			ProductCode:  "ITEM01",
			SrcUnitCode:  "BOX",
			DestUnitCode: "PC",
			ConvertQty:   2,
		},
		{
			ProductCode:  "ITEM01",
			SrcUnitCode:  "PC",
			DestUnitCode: "PC",
			ConvertQty:   1,
		},
		{
			ProductCode:  "ITEM02",
			SrcUnitCode:  "BOX",
			DestUnitCode: "PC",
			ConvertQty:   3,
		},
		{
			ProductCode:  "ITEM02",
			SrcUnitCode:  "PC",
			DestUnitCode: "PC",
			ConvertQty:   1,
		},
	}

	for _, item := range productConverter {
		key := fmt.Sprintf(`%s|%s`, item.ProductCode, item.SrcUnitCode)
		productConverterMap[key] = item
	}

	return productConverterMap, nil
}

func GetProduct(productsStr []string) (map[string]Product, error) {
	productMap := map[string]Product{}

	//Todo get from ProductMaster
	products := []Product{
		{
			ProductCode: "ITEM01",
			IsBatch:     false,
			IsSerial:    false,
		},
		{
			ProductCode: "ITEM02",
			IsBatch:     false,
			IsSerial:    false,
		},
	}

	for _, item := range products {
		key := item.ProductCode
		productMap[key] = item
	}

	return productMap, nil
}

func GetInventoryWarehouseConfig(sqlx *sqlx.DB, invWhConfig []InventoryWarehouseConfig) (map[string]InventoryWarehouseConfig, error) {
	invWhConfigMap := map[string]InventoryWarehouseConfig{}
	var condStr []string
	condStrCheck := map[string]bool{}

	for _, item := range invWhConfig {
		key := fmt.Sprintf(`%s|%s|%s|%s`, item.CompanyCode, item.SiteCode, item.WarehouseCode, item.ZoneCode)

		if _, exist := condStrCheck[key]; !exist {
			value := fmt.Sprintf(`('%s','%s','%s','%s')`, item.CompanyCode, item.SiteCode, item.WarehouseCode, item.ZoneCode)
			condStr = append(condStr, value)
			condStrCheck[key] = true
		}
	}

	if len(condStr) == 0 {
		return nil, fmt.Errorf(`not found config warehouse`)
	}

	query := fmt.Sprintf(`
		select c.company_code, s.site_code, w.warehouse_code, z.zone_code, z.is_negative_stock 
		from company c 
		left join site s on c.id = s.company_id 
		left join warehouse w on s.id  = w.site_id 
		left join "zone" z on w.id = z.warehouse_id 
		where c.company_code is not null and s.site_code is not null and w.warehouse_code is not null and z.zone_code is not null
		and (c.company_code, s.site_code, w.warehouse_code, z.zone_code) in (%s)
	`, strings.Join(condStr, `,`))
	//println(query)
	rows, err := db.ExecuteQuery(sqlx, query)
	if err != nil {
		return nil, err
	}

	for _, item := range rows {
		companyCode := item["company_code"].(string)
		siteCode := item["site_code"].(string)
		warehouseCode := item["warehouse_code"].(string)
		zoneCode := item["zone_code"].(string)
		isNegativeStock := item["is_negative_stock"].(bool)

		key := fmt.Sprintf(`%s|%s|%s|%s`, companyCode, siteCode, warehouseCode, zoneCode)
		invWhConfigMap[key] = InventoryWarehouseConfig{
			CompanyCode:     companyCode,
			SiteCode:        siteCode,
			WarehouseCode:   warehouseCode,
			ZoneCode:        zoneCode,
			IsNegativeStock: isNegativeStock,
		}
	}

	return invWhConfigMap, nil
}

func ConvertUnitInventoryTransaction(invTrans []InventoryTransaction, invTranSerial []InventoryTransactionSerial, productConverter map[string]ProductConverter) ([]InventoryTransaction, []InventoryTransactionSerial, error) {
	invTransMap := map[uuid.UUID]InventoryTransaction{}

	for i, item := range invTrans {
		key := fmt.Sprintf(`%s|%s`, item.ProductCode, item.UnitCode)

		if convert, exist := productConverter[key]; exist {
			editItem := item
			editItem.Qty = editItem.Qty * convert.ConvertQty
			editItem.UnitCode = convert.DestUnitCode

			invTrans[i] = editItem
		} else {
			return nil, nil, fmt.Errorf(`not found product or converter of product : %s`, item.ProductCode)
		}

		invTransMap[item.ID] = item
	}

	for i, item := range invTranSerial {
		if invTran, exist := invTransMap[item.TransactionID]; exist {
			key := fmt.Sprintf(`%s|%s`, invTran.ProductCode, item.UnitCode)

			if convert, exist := productConverter[key]; exist {
				editItem := item
				editItem.Qty = editItem.Qty * convert.ConvertQty
				editItem.UnitCode = convert.DestUnitCode

				invTranSerial[i] = editItem
			} else {
				return nil, nil, fmt.Errorf(`not found product or convert of product : %s`, invTran.ProductCode)
			}
		} else {
			return nil, nil, fmt.Errorf(`not found parent`)
		}
	}

	return invTrans, invTranSerial, nil
}

func CreateInventoryTransaction(gormx *gorm.DB, invTrans []InventoryTransaction, invTranSerial []InventoryTransactionSerial) error {

	if len(invTrans) == 0 {
		return fmt.Errorf(`not found data`)
	}

	tx := gormx.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	if len(invTrans) > 0 {
		err := tx.Create(&invTrans).Error
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	if len(invTranSerial) > 0 {
		err := tx.Create(&invTranSerial).Error
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func GetInventoryDB(gormx *gorm.DB, invTrans []InventoryTransaction) (map[string][]Inventory, error) {
	var inventory []Inventory
	inventoryMap := map[string][]Inventory{}
	var conditions []string
	var params []interface{}

	queryInv := gormx.Model(&Inventory{}).Where("qty <> 0")

	for _, trans := range invTrans {
		expDateStr := "IS NULL"
		mfgDateStr := "IS NULL"

		if trans.ExpDate != nil {
			expDateStr = "= ?"
		}

		if trans.MfgDate != nil {
			mfgDateStr = "= ?"
		}

		conditions = append(conditions, fmt.Sprintf(`
			(company_code = ? AND site_code = ? AND warehouse_code = ? AND zone_code = ? AND location_code = ? 
			AND pallet_code = ? AND container_code = ? AND batch_no = ? AND product_code = ? AND storage_type = ? 
			AND exp_date %s AND mfg_date %s)`,
			expDateStr, mfgDateStr,
		))

		params = append(params,
			trans.CompanyCode,
			trans.SiteCode,
			trans.WarehouseCode,
			trans.ZoneCode,
			trans.LocationCode,
			trans.PalletCode,
			trans.ContainerCode,
			trans.BatchNo,
			trans.ProductCode,
			trans.StorageType,
		)

		if trans.ExpDate != nil {
			expDate := trans.ExpDate.Truncate(24 * time.Hour)
			params = append(params, expDate.Format("2006-01-02"))
		}
		if trans.MfgDate != nil {
			mfgDate := trans.MfgDate.Truncate(24 * time.Hour)
			params = append(params, mfgDate.Format("2006-01-02"))
		}
	}

	if len(conditions) > 0 {
		queryInv = queryInv.Where(strings.Join(conditions, " OR "), params...)
	}

	if err := queryInv.Find(&inventory).Error; err != nil {
		return nil, err
	}

	for _, item := range inventory {
		expDateStr := ``
		mfgDateStr := ``

		if item.ExpDate != nil {
			expDate := item.ExpDate.Truncate(24 * time.Hour)
			expDateStr = expDate.Format("2006-01-02")
		}

		if item.MfgDate != nil {
			mfgDate := item.MfgDate.Truncate(24 * time.Hour)
			mfgDateStr = mfgDate.Format("2006-01-02")
		}

		key := fmt.Sprintf(`%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s`,
			item.CompanyCode, item.SiteCode, item.WarehouseCode, item.ZoneCode, item.LocationCode, item.PalletCode, item.ContainerCode, item.BatchNo, item.ProductCode, item.StorageType, expDateStr, mfgDateStr)
		inventoryMap[key] = append(inventoryMap[key], item)
	}

	return inventoryMap, nil
}

func BuildInvTransaction(datas []InventoryTransationRequest, action string, user string, tenantId uuid.UUID) ([]InventoryTransaction, []InventoryTransactionSerial, error) {
	invTrans := []InventoryTransaction{}
	invTransSerial := []InventoryTransactionSerial{}

	for _, item := range datas {
		invSrcTransId := uuid.New()
		invDestTransId := uuid.New()
		transactionType := `MOVE`
		srcCompanyCode := item.SrcCompanyCode
		srcSiteCode := item.SrcSiteCode
		srcWarehouseCode := item.SrcWarehouseCode
		srcZoneCode := item.SrcZoneCode
		srcLocationCode := item.SrcLocationCode
		srcPalletCode := item.SrcPalletCode
		srcContainerCode := item.SrcContainerCode
		destCompanyCode := item.DestCompanyCode
		destSiteCode := item.DestSiteCode
		destWarehouseCode := item.DestWarehouseCode
		destZoneCode := item.DestZoneCode
		destLocationCode := item.DestLocationCode
		destPalletCode := item.DestPalletCode
		destContainerCode := item.DestContainerCode
		storageType := item.StorageType
		documentRefType := item.DocumentRefType
		documentRef := item.DocumentRef
		itemRef := item.ItemRef
		batchNo := item.BatchNo
		productCode := item.ProductCode
		productType := ``
		qty := item.Qty
		unitCode := item.UnitCode
		status := `PENDING`
		mfgDate := item.MfgDate
		expDate := item.ExpDate
		createBy := user
		createDtm := time.Now()
		updateBy := user
		updateDtm := time.Now()

		//Normal & Batch Case
		newInvTrans := InventoryTransaction{
			ID:              invSrcTransId,
			TenantID:        tenantId,
			TransactionType: transactionType,
			CompanyCode:     srcCompanyCode,
			SiteCode:        srcSiteCode,
			WarehouseCode:   srcWarehouseCode,
			ZoneCode:        srcZoneCode,
			LocationCode:    srcLocationCode,
			PalletCode:      srcPalletCode,
			ContainerCode:   srcContainerCode,
			StorageType:     storageType,
			DocumentRefType: documentRefType,
			DocumentRef:     documentRef,
			ItemRef:         itemRef,
			ProductCode:     productCode,
			ProductType:     productType,
			Qty:             qty,
			UnitCode:        unitCode,
			BatchNo:         batchNo,
			Action:          `D`,
			Status:          status,
			MfgDate:         mfgDate,
			ExpDate:         expDate,
			CreateBy:        createBy,
			CreateDtm:       createDtm,
			UpdateBy:        updateBy,
			UpdateDtm:       updateDtm,
		}

		if action == `O` || action == `M` {
			invTrans = append(invTrans, newInvTrans)
		}

		newInvTrans.ID = invDestTransId
		newInvTrans.Action = `I`
		newInvTrans.CompanyCode = destCompanyCode
		newInvTrans.SiteCode = destSiteCode
		newInvTrans.WarehouseCode = destWarehouseCode
		newInvTrans.ZoneCode = destZoneCode
		newInvTrans.LocationCode = destLocationCode
		newInvTrans.PalletCode = destPalletCode
		newInvTrans.ContainerCode = destContainerCode

		if action == `I` || action == `M` {
			invTrans = append(invTrans, newInvTrans)
		}

		//Serial Case
		if len(item.Serials) > 0 {
			for _, itemSerial := range item.Serials {
				newSrcInvTransId := invSrcTransId
				newDestInvTransId := invDestTransId

				newInvTransSerial := InventoryTransactionSerial{
					ID:            uuid.New(),
					TransactionID: newSrcInvTransId,
					SerialCode:    itemSerial.SerialNo,
					Qty:           itemSerial.Qty,
					UnitCode:      itemSerial.UnitCode,
					MfgDate:       itemSerial.MfgDate,
					ExpDate:       itemSerial.ExpDate,
				}

				if action == `O` || action == `M` {
					invTransSerial = append(invTransSerial, newInvTransSerial)
				}

				newInvTransSerial.ID = uuid.New()
				newInvTransSerial.TransactionID = newDestInvTransId

				if action == `I` || action == `M` {
					invTransSerial = append(invTransSerial, newInvTransSerial)
				}
			}
		}
	}

	return invTrans, invTransSerial, nil
}

func ValidateInvTransaction(action string, inventoryMap map[string][]Inventory, invTrans []InventoryTransaction, invTransSerial []InventoryTransactionSerial, productMap map[string]Product, productConverterMap map[string]ProductConverter, invWhConfigMap map[string]InventoryWarehouseConfig) error {
	type SummaryInventory struct {
		CompanyCode     string
		SiteCode        string
		WarehouseCode   string
		ZoneCode        string
		LocationCode    string
		PalletCode      string
		ContainerCode   string
		BatchNo         string
		ProductCode     string
		StorageType     string
		ExpDate         *time.Time
		MfgDate         *time.Time
		InvQty          float64
		InvUnitCode     string
		RequestQty      float64
		RequestUnitCode string
	}

	invTransSerialMap := map[string][]InventoryTransactionSerial{}
	summaryInvMap := map[string]SummaryInventory{}

	for _, item := range invTransSerial {
		key := item.TransactionID.String()
		invTransSerialMap[key] = append(invTransSerialMap[key], item)
	}

	for _, item := range invTrans {
		companyCode := item.CompanyCode
		siteCode := item.SiteCode
		warehouseCode := item.WarehouseCode
		zoneCode := item.ZoneCode
		locationCode := item.LocationCode
		palletCode := item.PalletCode
		containerCode := item.ContainerCode
		batchNo := item.BatchNo
		productCode := item.ProductCode
		storageType := item.StorageType
		expDate := item.ExpDate
		mfgDate := item.MfgDate
		qty := item.Qty
		unitCode := item.UnitCode
		expDateStr := ``
		mfgDateStr := ``

		if item.ExpDate != nil {
			expDate := item.ExpDate.Truncate(24 * time.Hour)
			expDateStr = expDate.Format("2006-01-02")
		}

		if item.MfgDate != nil {
			mfgDate := item.MfgDate.Truncate(24 * time.Hour)
			mfgDateStr = mfgDate.Format("2006-01-02")
		}

		invKey := fmt.Sprintf(`%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s`,
			item.CompanyCode, item.SiteCode, item.WarehouseCode, item.ZoneCode, item.LocationCode, item.PalletCode, item.ContainerCode, item.BatchNo, item.ProductCode, item.StorageType, expDateStr, mfgDateStr)

		if sumInv, exist := summaryInvMap[invKey]; exist {
			editItem := sumInv
			editItem.RequestQty += qty

			summaryInvMap[invKey] = editItem

			continue
		}

		summaryInvMap[invKey] = SummaryInventory{
			CompanyCode:     companyCode,
			SiteCode:        siteCode,
			WarehouseCode:   warehouseCode,
			ZoneCode:        zoneCode,
			LocationCode:    locationCode,
			PalletCode:      palletCode,
			ContainerCode:   containerCode,
			BatchNo:         batchNo,
			ProductCode:     productCode,
			StorageType:     storageType,
			ExpDate:         expDate,
			MfgDate:         mfgDate,
			RequestQty:      qty,
			RequestUnitCode: unitCode,
		}
	}

	for _, inv := range inventoryMap {
		for _, item := range inv {
			qty := item.Qty
			expDateStr := ``
			mfgDateStr := ``

			if item.ExpDate != nil {
				expDate := item.ExpDate.Truncate(24 * time.Hour)
				expDateStr = expDate.Format("2006-01-02")
			}

			if item.MfgDate != nil {
				mfgDate := item.MfgDate.Truncate(24 * time.Hour)
				mfgDateStr = mfgDate.Format("2006-01-02")
			}

			invKey := fmt.Sprintf(`%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s`,
				item.CompanyCode, item.SiteCode, item.WarehouseCode, item.ZoneCode, item.LocationCode, item.PalletCode, item.ContainerCode, item.BatchNo, item.ProductCode, item.StorageType, expDateStr, mfgDateStr)

			if sumInv, exist := summaryInvMap[invKey]; exist {
				editItem := sumInv
				editItem.InvQty += qty

				summaryInvMap[invKey] = editItem
			}
		}
	}

	//validate
	for _, invTran := range invTrans {
		companyCode := invTran.CompanyCode
		siteCode := invTran.SiteCode
		warehouseCode := invTran.WarehouseCode
		zoneCode := invTran.ZoneCode
		locationCode := invTran.LocationCode
		palletCode := invTran.PalletCode
		containerCode := invTran.ContainerCode
		batchNo := invTran.BatchNo
		productCode := invTran.ProductCode
		storageType := invTran.StorageType
		expDateStr := ``
		mfgDateStr := ``

		if invTran.ExpDate != nil {
			expDate := invTran.ExpDate.Truncate(24 * time.Hour)
			expDateStr = expDate.Format("2006-01-02")
		}

		if invTran.MfgDate != nil {
			mfgDate := invTran.MfgDate.Truncate(24 * time.Hour)
			mfgDateStr = mfgDate.Format("2006-01-02")
		}

		warehouseKey := fmt.Sprintf(`%s|%s|%s|%s`, companyCode, siteCode, warehouseCode, zoneCode)

		//validate warehouse
		warehouseConf, exist := invWhConfigMap[warehouseKey]
		if !exist {
			return fmt.Errorf(`not found warehouse config of company: %s, site: %s, warehouse: %s, zone: %s`, companyCode, siteCode, warehouseCode, zoneCode)
		}

		//validate src inventory
		if !warehouseConf.IsNegativeStock && (action == `O` || action == `M`) {
			invKey := fmt.Sprintf(`%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s`,
				companyCode, siteCode, warehouseCode, zoneCode, locationCode, palletCode, containerCode, batchNo, productCode, storageType, expDateStr, mfgDateStr)

			sumInvTran, exist := summaryInvMap[invKey]
			if !exist {
				return fmt.Errorf(`not found inventory transaction of company: %s, site: %s, warehouse: %s, zone: %s`, companyCode, siteCode, warehouseCode, zoneCode)
			}

			if sumInvTran.RequestQty > sumInvTran.InvQty {
				return fmt.Errorf(`not avalible stock for movement of company: %s, site: %s, warehouse: %s, zone: %s`, companyCode, siteCode, warehouseCode, zoneCode)
			}
		}

		//validate product
		productKey := invTran.ProductCode
		if product, exist := productMap[productKey]; exist {
			isBatch := product.IsBatch
			isSerial := product.IsSerial
			serialKey := invTran.ID.String()

			invTranSerial, exist := invTransSerialMap[serialKey]

			if isBatch && invTran.BatchNo == `` {
				return fmt.Errorf(`missing batch product: %s `, invTran.ProductCode)
			}

			if isSerial {
				if !exist || len(invTranSerial) == 0 {
					return fmt.Errorf(`missing serial product: %s `, invTran.ProductCode)
				}

				if invTran.Qty != float64(len(invTranSerial)) {
					return fmt.Errorf(`missing qty serial product: %s `, invTran.ProductCode)
				}
			} else if len(invTranSerial) > 0 {
				return fmt.Errorf(`not found serial in product type serial of product: %s`, invTran.ProductCode)
			}
		} else {
			return fmt.Errorf(`not found product master: %s`, invTran.ProductCode)
		}
	}

	return nil
}

func BuildInv(inventoryMap map[string][]Inventory, invTrans []InventoryTransaction, invTransSerial []InventoryTransactionSerial, user string, tenantId uuid.UUID) ([]Inventory, []Inventory, []InventorySerial, []InventorySerial, error) {
	var createInv []Inventory
	var updateInv []Inventory
	var createInvSerial []InventorySerial
	var deleteInvSerial []InventorySerial
	invTransSerialMap := map[string][]InventoryTransactionSerial{}

	for _, item := range invTransSerial {
		key := item.TransactionID.String()
		invTransSerialMap[key] = append(invTransSerialMap[key], item)
	}

	for _, invTrans := range invTrans {
		action := invTrans.Action
		expDateStr := ``
		mfgDateStr := ``

		if invTrans.ExpDate != nil {
			expDate := invTrans.ExpDate.Truncate(24 * time.Hour)
			expDateStr = expDate.Format("2006-01-02")
		}

		if invTrans.MfgDate != nil {
			mfgDate := invTrans.MfgDate.Truncate(24 * time.Hour)
			mfgDateStr = mfgDate.Format("2006-01-02")
		}

		invKey := fmt.Sprintf(`%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s`,
			invTrans.CompanyCode, invTrans.SiteCode, invTrans.WarehouseCode, invTrans.ZoneCode, invTrans.LocationCode, invTrans.PalletCode, invTrans.ContainerCode, invTrans.BatchNo, invTrans.ProductCode, invTrans.StorageType, expDateStr, mfgDateStr)
		invArr, existInv := inventoryMap[invKey]
		//Todo sort invArr for FIFO เพิ่ม 1 column ให้เป็น last_Increese และเอาเวลานั้นมาเรียง

		if action == `I` {
			isFound := false

			//find same key for update qty
			if existInv {
				for _, inv := range invArr {
					if updateMatchInventoryWithTransaction(inv, invTrans) {
						invTemp := inv
						invTemp.Qty = invTrans.Qty
						invTemp.UpdateBy = user
						invTemp.UpdateDtm = time.Now()

						updateInv = append(updateInv, invTemp)

						invTransSerialKey := invTrans.ID.String()
						if invTransSerial, exist := invTransSerialMap[invTransSerialKey]; exist {
							for _, invSerial := range invTransSerial {
								invSerialTemp := InventorySerial{
									ID:          uuid.New(),
									InventoryID: invTemp.ID,
									SerialCode:  invSerial.SerialCode,
									Qty:         invSerial.Qty,
									UnitCode:    invSerial.UnitCode,
									MfgDate:     invSerial.MfgDate,
									ExpDate:     invSerial.ExpDate,
									CreateBy:    user,
									CreateDtm:   time.Now(),
									UpdateBy:    user,
									UpdateDtm:   time.Now(),
								}

								createInvSerial = append(createInvSerial, invSerialTemp)
							}
						}

						isFound = true
						break
					}
				}
			}

			//after find if not found then create new row
			if !isFound {
				tempInvId := uuid.New()

				invTemp := Inventory{
					ID:            tempInvId,
					TenantID:      tenantId,
					StorageType:   invTrans.StorageType,
					CompanyCode:   invTrans.CompanyCode,
					SiteCode:      invTrans.SiteCode,
					WarehouseCode: invTrans.WarehouseCode,
					ZoneCode:      invTrans.ZoneCode,
					LocationCode:  invTrans.LocationCode,
					PalletCode:    invTrans.PalletCode,
					ContainerCode: invTrans.ContainerCode,
					BatchNo:       invTrans.BatchNo,
					ProductCode:   invTrans.ProductCode,
					Qty:           invTrans.Qty,
					UnitCode:      invTrans.UnitCode,
					MfgDate:       invTrans.MfgDate,
					ExpDate:       invTrans.ExpDate,
					CreateBy:      user,
					CreateDtm:     time.Now(),
					UpdateBy:      user,
					UpdateDtm:     time.Now(),
				}

				createInv = append(createInv, invTemp)

				invTransSerialKey := invTrans.ID.String()
				if invTransSerial, exist := invTransSerialMap[invTransSerialKey]; exist {
					for _, invSerial := range invTransSerial {
						invSerialTemp := InventorySerial{
							ID:          uuid.New(),
							InventoryID: invTemp.ID,
							SerialCode:  invSerial.SerialCode,
							Qty:         invSerial.Qty,
							UnitCode:    invSerial.UnitCode,
							MfgDate:     invSerial.MfgDate,
							ExpDate:     invSerial.ExpDate,
							CreateBy:    user,
							CreateDtm:   time.Now(),
							UpdateBy:    user,
							UpdateDtm:   time.Now(),
						}

						createInvSerial = append(createInvSerial, invSerialTemp)
					}
				}
			}
		} else if action == `D` {
			isFound := false
			remainInvTransQty := invTrans.Qty

			if existInv {
				for _, inv := range invArr {
					if updateMatchInventoryWithTransaction(inv, invTrans) {

						invTempQty := remainInvTransQty

						if invTempQty > inv.Qty {
							invTempQty = inv.Qty

							if inv.Qty < 0 {
								invTempQty = -math.Abs(remainInvTransQty)
							}
						}

						invTemp := inv
						invTemp.Qty = -math.Abs(invTempQty)
						invTemp.UpdateBy = user
						invTemp.UpdateDtm = time.Now()

						updateInv = append(updateInv, invTemp)

						invTransSerialKey := invTrans.ID.String()
						if invTransSerial, exist := invTransSerialMap[invTransSerialKey]; exist {
							for _, invSerial := range invTransSerial {
								invSerialTemp := InventorySerial{
									InventoryID: invTemp.ID,
									SerialCode:  invSerial.SerialCode,
								}

								deleteInvSerial = append(deleteInvSerial, invSerialTemp)
							}
						}

						remainInvTransQty -= math.Abs(invTempQty)

						if remainInvTransQty <= 0 {
							isFound = true
							break
						}
					}
				}
			}

			//after find. if not found then create new row
			if !isFound {
				tempInvId := uuid.New()

				invTemp := Inventory{
					ID:            tempInvId,
					TenantID:      tenantId,
					StorageType:   invTrans.StorageType,
					CompanyCode:   invTrans.CompanyCode,
					SiteCode:      invTrans.SiteCode,
					WarehouseCode: invTrans.WarehouseCode,
					ZoneCode:      invTrans.ZoneCode,
					LocationCode:  invTrans.LocationCode,
					PalletCode:    invTrans.PalletCode,
					ContainerCode: invTrans.ContainerCode,
					BatchNo:       invTrans.BatchNo,
					ProductCode:   invTrans.ProductCode,
					Qty:           -math.Abs(remainInvTransQty),
					UnitCode:      invTrans.UnitCode,
					MfgDate:       invTrans.MfgDate,
					ExpDate:       invTrans.ExpDate,
					CreateBy:      user,
					CreateDtm:     time.Now(),
					UpdateBy:      user,
					UpdateDtm:     time.Now(),
				}

				createInv = append(createInv, invTemp)
			}
		}
	}

	return createInv, updateInv, createInvSerial, deleteInvSerial, nil
}

func UpdateInv(gormx *gorm.DB, createInv []Inventory, updateInv []Inventory, createInvSerial []InventorySerial, deleteInvSerial []InventorySerial, user string, tenantId uuid.UUID) error {
	tx := gormx.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	//Delte Invetory Serial
	if len(deleteInvSerial) > 0 {
		var conditions []string
		var values []interface{}

		for _, serial := range deleteInvSerial {
			conditions = append(conditions, "?")
			values = append(values, serial.SerialCode)
		}

		query := fmt.Sprintf("serial_code IN (%s)", strings.Join(conditions, ","))

		err := tx.Where(query, values...).Delete(&InventorySerial{}).Error
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	//Create Inventory
	if len(createInv) > 0 {
		err := tx.Create(&createInv).Error
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	//Update Inventory
	if len(updateInv) > 0 {
		sql := "UPDATE inventory SET qty = CASE "
		var ids []interface{}
		var condition []interface{}

		for _, inv := range updateInv {
			sql += " WHEN id = ? THEN qty + ? "
			ids = append(ids, inv.ID, inv.Qty)
			condition = append(condition, inv.ID)
		}

		sql += " END "
		sql += " , update_by = ?, update_dtm = ? "
		sql += "  WHERE id IN (?) "

		ids = append(ids, user, time.Now(), condition)

		if err := tx.Exec(sql, ids...).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	//Create Inventory Serial
	if len(createInvSerial) > 0 {
		err := tx.Create(&createInvSerial).Error
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func DeleteZeroInventory(gormx *gorm.DB, inventoryIds []uuid.UUID) error {
	var inventory []Inventory
	var deleteInventortIds []uuid.UUID

	if len(inventoryIds) == 0 {
		return nil
	}

	tx := gormx.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	if err := tx.Find(&inventory).Where("id in (?) and qty = 0", inventoryIds).Error; err != nil {
		return err
	}

	for _, item := range inventory {
		if item.Qty == 0 {
			deleteInventortIds = append(deleteInventortIds, item.ID)
		}
	}

	if len(deleteInventortIds) > 0 {
		err := tx.Where("id in (?)", deleteInventortIds).Delete(&Inventory{}).Error
		if err != nil {
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func updateMatchInventoryWithTransaction(inventory Inventory, trans InventoryTransaction) bool {
	return inventory.CompanyCode == trans.CompanyCode &&
		inventory.SiteCode == trans.SiteCode &&
		inventory.WarehouseCode == trans.WarehouseCode &&
		inventory.ZoneCode == trans.ZoneCode &&
		inventory.LocationCode == trans.LocationCode &&
		inventory.PalletCode == trans.PalletCode &&
		inventory.ContainerCode == trans.ContainerCode &&
		inventory.BatchNo == trans.BatchNo &&
		inventory.ProductCode == trans.ProductCode &&
		inventory.StorageType == trans.StorageType &&
		inventory.UnitCode == trans.UnitCode
}
