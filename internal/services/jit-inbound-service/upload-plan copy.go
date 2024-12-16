package jitInboundService

// import (
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"jnv-jit/internal/db"
// 	"math"
// 	"sort"
// 	"strings"
// 	"time"

// 	"github.com/gin-gonic/gin"
// 	"github.com/jmoiron/sqlx"
// 	"gorm.io/gorm"
// )

// type UploadPlanRequest struct {
// 	IsBom          bool            `json:"is_bom"`
// 	IsCheckFg      bool            `json:"is_check_fg"`
// 	MaterialStocks []MaterialStock `json:"material_stocks"`
// 	RequestPlan    []RequestPlan   `json:"request_plan"`
// }

// type RequestPlan struct {
// 	MaterialCode     string    `json:"material_code"`
// 	LineCode         string    `json:"line_code"`
// 	RequestQty       float64   `json:"request_qty"`
// 	RequestPlantQty  float64   `json:"request_plant_qty"`
// 	RequestSubconQty float64   `json:"request_subcon_qty"`
// 	PlanDate         time.Time `json:"plan_date"`
// }

// type MaterialStock struct {
// 	MaterialCode   string  `json:"material_code"`
// 	StockPlantQty  float64 `json:"stock_plant_qty"`
// 	StockSubconQty float64 `json:"stock_subcon_qty"`
// }

// func UploadPlan(c *gin.Context, jsonPayload string) (interface{}, error) {
// 	var req UploadPlanRequest

// 	if err := json.Unmarshal([]byte(jsonPayload), &req); err != nil {
// 		return nil, errors.New("failed to unmarshal JSON into struct: " + err.Error())
// 	}

// 	_, err := ProcessUploadPlan(req)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return nil, nil
// }

// func ProcessUploadPlan(req UploadPlanRequest) (interface{}, error) {
// 	var reqPlan []RequestPlan
// 	var reqMatStock []MaterialStock
// 	reqPlan = req.RequestPlan
// 	reqMatStock = req.MaterialStocks

// 	gormx, err := db.ConnectGORM(`jit_portal`)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer db.CloseGORM(gormx)

// 	sqlx, err := db.ConnectSqlx(`jit_portal`)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer sqlx.Close()

// 	startDate := GetStartDate()
// 	endDate := time.Now().Truncate(24 * time.Hour)
// 	isBom := req.IsBom
// 	isCheckFg := req.IsCheckFg
// 	matLineMap := map[string]map[string]MaterialLine{}
// 	var mats []Material
// 	matCheck := map[string]bool{}
// 	reqMap := map[string][]RequestPlan{}

// 	for i, item := range reqPlan {
// 		materialCode := item.MaterialCode
// 		lineCode := item.LineCode
// 		requestPlanQty := item.RequestPlantQty
// 		requestSubconQty := item.RequestSubconQty
// 		planDate := item.PlanDate.Truncate(24 * time.Hour)
// 		planDateStr := planDate.Format("2006-01-02")

// 		newReq := item
// 		newReq.RequestQty += requestPlanQty + requestSubconQty

// 		resKey := fmt.Sprintf(`%s|%s|%s`, planDateStr, materialCode, lineCode)
// 		reqMap[resKey] = append(reqMap[resKey], newReq)

// 		if i == 0 || endDate.Before(newReq.PlanDate) {
// 			endDate = newReq.PlanDate
// 		}

// 		materialKey := materialCode
// 		materialLineKey := fmt.Sprintf(`%s|%s`, materialCode, lineCode)
// 		if _, exist := matLineMap[materialKey]; !exist {
// 			matLineMap[materialKey] = map[string]MaterialLine{}
// 		}

// 		matLineMap[materialKey][materialLineKey] = MaterialLine{
// 			MaterialCode: materialCode,
// 			LineCode:     lineCode,
// 		}

// 		matKey := materialCode
// 		if _, exist := matCheck[matKey]; !exist {
// 			mats = append(mats, Material{
// 				MaterialCode: materialCode,
// 			})

// 			matCheck[matKey] = true
// 		}
// 	}

// 	matBompMap, err := GetBom(sqlx, mats, isBom)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get BOM: %w", err)
// 	}

// 	if isBom {

// 		if err := ValidationBom(reqMap, matBompMap); err != nil {
// 			return nil, fmt.Errorf("failed to validation bom: %w", err)
// 		}
// 	}

// 	if isCheckFg {
// 		if err := ValidationFg(reqMap, matBompMap); err != nil {
// 			return nil, fmt.Errorf("failed to validation fg: %w", err)
// 		}
// 	}

// 	//jitDailyPlan, jitDailyMap, err := BuildJitDaily(startDate, endDate, matLineMap, reqMap, matBompMap)
// 	_, jitDailyMap, err := BuildJitDaily(startDate, endDate, matLineMap, reqMap, matBompMap)
// 	if err != nil {
// 		return nil, fmt.Errorf("error building JIT daily map: %w", err)
// 	}

// 	bomMats := []string{}
// 	bomMatCheck := map[string]bool{}
// 	for _, jitLines := range jitDailyMap {
// 		for _, item := range jitLines {
// 			materialCode := item.MaterialCode

// 			key := materialCode
// 			if _, exist := bomMatCheck[key]; !exist {
// 				bomMats = append(bomMats, materialCode)

// 				bomMatCheck[key] = true
// 			}
// 		}
// 	}

// 	jitDailyDBMap, err := GetJitDailyDB(sqlx, startDate, bomMats)
// 	if err != nil {
// 		return nil, fmt.Errorf("error get jit daily db: %w", err)
// 	}

// 	//todo loop merge เพราะข้อมูลเปลี่ยนเป็น array แล้ว
// 	jitDailyMap, err = MergeJitDaily(jitDailyMap, jitDailyDBMap)
// 	if err != nil {
// 		return nil, fmt.Errorf("error merge jit daily: %w", err)
// 	}

// 	//jitMats, confirmJitDateMap, err := CalculateEstimate(jitDailyMap, reqMatStock, startDate)
// 	//todo จังหวะเกิด require จะต้องไปสร้าง require โดยอ้างอิงจาก planId ของ prod ด้วย
// 	jitMats, _, materialCodes, lineCodes, err := CalculateEstimate(jitDailyMap, reqMatStock, startDate)
// 	if err != nil {
// 		return nil, fmt.Errorf("error calculate estimate: %w", err)
// 	}

// 	// jitMats, err = CalculateActual(jitMats, confirmJitDateMap)
// 	// if err != nil {
// 	// 	return nil, fmt.Errorf("error calculate actual: %w", err)
// 	// }

// 	lineMap, err := GetLineMap(sqlx, lineCodes)
// 	if err != nil {
// 		return nil, err
// 	}

// 	materialMap, err := GetMatrialMap(sqlx, materialCodes)
// 	if err != nil {
// 		return nil, err
// 	}

// 	jitDailys, err := ConvertToJitDailyDB(jitMats, lineMap, materialMap)
// 	if err != nil {
// 		return nil, fmt.Errorf("error convert jit daily db: %w", err)
// 	}

// 	err = CreateJitDaily(gormx, jitDailys, startDate)
// 	if err != nil {
// 		return nil, fmt.Errorf("error create jit daily: %w", err)
// 	}

// 	return nil, nil
// }

// func GetStartDate() time.Time {
// 	startDate := time.Now().Truncate(24 * time.Hour)

// 	//todo Cal start date

// 	return startDate
// }

// func GetBom(sqlx *sqlx.DB, mats []Material, isBom bool) (map[string]Material, error) {
// 	matMap := map[string]Material{}

// 	var matStr []string
// 	matStrCheck := map[string]bool{}

// 	for _, item := range mats {
// 		key := item.MaterialCode

// 		if _, exist := matStrCheck[key]; !exist {
// 			matStr = append(matStr, item.MaterialCode)

// 			matStrCheck[key] = true
// 		}
// 	}

// 	if len(matStr) == 0 {
// 		return nil, fmt.Errorf(`not found material`)
// 	}

// 	query := fmt.Sprintf(`
// 		select m.material_id as material_id, m.material_code as material_code
// 			, m.supplier_id as supplier_id, coalesce(m.delivery_lead_time,0) as material_lead_time
// 			, coalesce(s.supplier_code, '') as supplier_code
// 			, coalesce(jm.fb_material_id, 0) as bom_id, coalesce(jm.fg_per_fb,0) as bom_qty, coalesce(jm.waste,0) as bom_waste
// 			, coalesce(mb.material_code, '') as bom_code,  coalesce(mb.delivery_lead_time,0) as bom_lead_time
// 		from materials m
// 		left join jit_master jm on m.material_id  = jm.fg_material_id
// 		left join suppliers s on m.supplier_id = s.supplier_id
// 		left join materials mb on mb.material_id = jm.fb_material_id
// 		where 1=1
// 		and m.material_code in ('%s')
// 	`, strings.Join(matStr, `','`))
// 	//println(query)
// 	rows, err := db.ExecuteQuery(sqlx, query)
// 	if err != nil {
// 		return nil, err
// 	}

// 	for _, item := range rows {
// 		materialId := item["material_id"].(int64)
// 		materialCode := item["material_code"].(string)
// 		supplierId := item["supplier_id"].(int64)
// 		supplierCode := item["supplier_code"].(string)
// 		bomId := item["bom_id"].(int64)
// 		bomCode := item["bom_code"].(string)
// 		bomQty := item["bom_qty"].(float64)
// 		materialLeadTime := int64(item["material_lead_time"].(float64))
// 		bomLeadTime := int64(item["bom_lead_time"].(float64))
// 		bomWaste := item["bom_waste"].(float64)

// 		key := materialCode

// 		mat, exist := matMap[key]
// 		if !exist {
// 			mat = Material{
// 				MaterialId:   materialId,
// 				MaterialCode: materialCode,
// 				SupplierId:   supplierId,
// 				SupplierCode: supplierCode,
// 				Qty:          1,
// 				LeadTime:     materialLeadTime,
// 			}
// 		}

// 		if bomId != 0 && isBom {
// 			mat.Boms = append(mat.Boms, Bom{
// 				MaterialId:   bomId,
// 				MaterialCode: bomCode,
// 				LeadTime:     bomLeadTime,
// 				Qty:          bomQty,
// 				Waste:        bomWaste,
// 			})
// 		}

// 		matMap[key] = mat
// 	}

// 	return matMap, nil
// }

// func ValidationBom(planMap map[string][]RequestPlan, matBompMap map[string]Material) error {
// 	for _, plan := range planMap {
// 		for _, item := range plan {
// 			materialCode := item.MaterialCode

// 			matBom, exist := matBompMap[materialCode]
// 			if !exist || len(matBom.Boms) == 0 {
// 				return fmt.Errorf(`not found bom of material code : %s`, materialCode)
// 			}
// 		}
// 	}

// 	return nil
// }

// func ValidationFg(planMap map[string][]RequestPlan, matBompMap map[string]Material) error {
// 	for _, plan := range planMap {
// 		for _, item := range plan {
// 			materialCode := item.MaterialCode

// 			if _, exist := matBompMap[materialCode]; !exist {
// 				return fmt.Errorf(`not found bom of material code : %s`, materialCode)
// 			}
// 		}
// 	}

// 	return nil
// }

// func BuildJitDaily(startDate time.Time, endDate time.Time, matLineMap map[string]map[string]MaterialLine, datas map[string][]RequestPlan, matBomMap map[string]Material) ([]JitDilyPlan, map[string][]JitLine, error) {
// 	jitLineMap := map[string][]JitLine{}
// 	JitDilyPlans := []JitDilyPlan{}
// 	planIdCount := int64(1)

// 	for currentDate := startDate; !currentDate.After(endDate); currentDate = currentDate.Add(24 * time.Hour) {
// 		for _, mat := range matLineMap {
// 			for _, matLine := range mat {
// 				planDateStr := currentDate.Format("2006-01-02")
// 				materialCode := matLine.MaterialCode
// 				lineCode := matLine.LineCode
// 				jitLineKey := fmt.Sprintf(`%s|%s|%s`, planDateStr, materialCode, lineCode)

// 				matBomKey := materialCode
// 				matBom, matBomExist := matBomMap[matBomKey]
// 				if !matBomExist {
// 					continue
// 				}

// 				dataItems, dataItemExists := datas[jitLineKey]
// 				if dataItemExists {
// 					for _, dataItem := range dataItems {
// 						planId := planIdCount
// 						materialCode := dataItem.MaterialCode
// 						lineCode := dataItem.LineCode
// 						requestQty := dataItem.RequestQty
// 						requestPlanQty := dataItem.RequestPlantQty
// 						requestSubconQty := dataItem.RequestSubconQty

// 						JitDilyPlans = append(JitDilyPlans, JitDilyPlan{
// 							PlanId:           planId,
// 							MaterialCode:     materialCode,
// 							LineCode:         lineCode,
// 							RequestQty:       requestQty,
// 							RequestPlantQty:  requestPlanQty,
// 							RequestSubconQty: requestSubconQty,
// 							PlanDate:         currentDate,
// 						})

// 						if len(matBom.Boms) > 0 {
// 							for _, bom := range matBom.Boms {
// 								bomMaterialCode := bom.MaterialCode
// 								waste := bom.Waste
// 								bomLeadTime := bom.LeadTime

// 								jitLineBomKey := fmt.Sprintf(`%s|%s|%s`, planDateStr, bomMaterialCode, lineCode)

// 								jitLine := JitLine{
// 									id:                  0,
// 									PlanId:              planId,
// 									DailyDate:           currentDate,
// 									MaterialCode:        bomMaterialCode,
// 									LineCode:            lineCode,
// 									ProductionQty:       requestQty,
// 									ProductionPlantQty:  requestPlanQty,
// 									ProductionSubconQty: requestSubconQty,
// 									RequireQty:          0,
// 									UrgenQty:            0,
// 									LeadTime:            bomLeadTime,
// 									RefReuqestID:        nil,
// 								}

// 								if jitLine.ProductionQty == 0 || bom.Qty == 0 {
// 									return nil, nil, fmt.Errorf(`productionQty or bom.qty = 0`)
// 								}

// 								ProductionQty := (jitLine.ProductionQty / bom.Qty)

// 								if waste != 0 {
// 									ProductionQty *= (1 + waste/100)
// 								}

// 								jitLine.ProductionQty = ProductionQty
// 								jitLineMap[jitLineBomKey] = append(jitLineMap[jitLineBomKey], jitLine)
// 							}
// 						} else {
// 							materialLeadTime := matBom.LeadTime

// 							jitLine := JitLine{
// 								id:                  0,
// 								PlanId:              planId,
// 								DailyDate:           currentDate,
// 								MaterialCode:        materialCode,
// 								LineCode:            lineCode,
// 								ProductionQty:       requestQty,
// 								ProductionPlantQty:  requestPlanQty,
// 								ProductionSubconQty: requestSubconQty,
// 								RequireQty:          0,
// 								UrgenQty:            0,
// 								LeadTime:            materialLeadTime,
// 								RefReuqestID:        nil,
// 							}

// 							jitLineMap[jitLineKey] = append(jitLineMap[jitLineKey], jitLine)
// 						}

// 						planIdCount++
// 					}
// 				} else {
// 					if len(matBom.Boms) > 0 {
// 						for _, bom := range matBom.Boms {
// 							bomMaterialCode := bom.MaterialCode
// 							jitLineBomKey := fmt.Sprintf(`%s|%s|%s`, planDateStr, bomMaterialCode, lineCode)

// 							jitLine := JitLine{
// 								id:                  0,
// 								DailyDate:           currentDate,
// 								MaterialCode:        bomMaterialCode,
// 								LineCode:            lineCode,
// 								ProductionQty:       0,
// 								ProductionPlantQty:  0,
// 								ProductionSubconQty: 0,
// 								RequireQty:          0,
// 								UrgenQty:            0,
// 								LeadTime:            bom.LeadTime,
// 								RefReuqestID:        nil,
// 							}

// 							jitLineMap[jitLineBomKey] = append(jitLineMap[jitLineBomKey], jitLine)
// 						}
// 					} else {
// 						materialLeadTime := matBom.LeadTime

// 						jitLine := JitLine{
// 							id:                  0,
// 							DailyDate:           currentDate,
// 							MaterialCode:        materialCode,
// 							LineCode:            lineCode,
// 							ProductionQty:       0,
// 							ProductionPlantQty:  0,
// 							ProductionSubconQty: 0,
// 							RequireQty:          0,
// 							UrgenQty:            0,
// 							LeadTime:            materialLeadTime,
// 							RefReuqestID:        nil,
// 						}

// 						jitLineMap[jitLineKey] = append(jitLineMap[jitLineKey], jitLine)
// 					}
// 				}
// 			}
// 		}
// 	}

// 	return JitDilyPlans, jitLineMap, nil
// }

// func GetJitDailyDB(sqlx *sqlx.DB, startDate time.Time, matStrs []string) (map[string][]JitLine, error) {
// 	jitDailyMap := map[string][]JitLine{}

// 	query := fmt.Sprintf(`
// 		select jd.jit_daily_id as id
// 			, jd.jit_daily_plan_id  as plan_id
// 			, jd.original_jit_daily_id as ref_request_id
// 			, jd.daily_date
// 			, coalesce(jd.product_qty, 0) as product_qty
// 			, coalesce(jd.plant_qty , 0) as product_plant_qty
// 			, coalesce(jd.subcon_qty , 0) as product_subcon_qty
// 			, coalesce(jd.required_qty, 0)  as require_qty
// 			, coalesce(jd.urgent_qty, 0) as urgent_qty
// 			, coalesce(jd.conf_qty ,0) as confirm_require_qty
// 			, coalesce(jd.conf_urgent_qty ,0) as confirm_urgent_qty
// 			, jd.conf_date as confirm_require_date
// 			, jd.conf_urgent_date as confirm_urgent_date
// 			, m.material_code
// 			, jlh.line_header_name as line_code
// 			, coalesce(m.delivery_lead_time,0) as delivery_lead_time
// 		from jit_daily jd
// 		left join materials m on jd.material_id = m.material_id
// 		left join jit_line_details jld on jld.line_detail_id  = jd.line_id
// 		left join jit_line_headers jlh on jld.line_header_id = jlh.line_header_id
// 		where 1=1
// 		and jd.daily_date >= '%s'
// 		and m.material_code in ('%s')
// 	`, startDate.Format("2006-01-02"), strings.Join(matStrs, `','`))
// 	//println(query)
// 	rows, err := db.ExecuteQuery(sqlx, query)
// 	if err != nil {
// 		return nil, err
// 	}

// 	for _, item := range rows {
// 		id := item["id"].(int64)
// 		refRequestId := item["ref_request_id"].(int64)
// 		planId := item["plan_id"].(int64)
// 		planDate := item["daily_date"].(time.Time).Truncate(24 * time.Hour)
// 		planDateStr := planDate.Format("2006-01-02")
// 		materialCode := item["material_code"].(string)
// 		lineCode := item["line_code"].(string)
// 		productionQty := item["production_qty"].(float64)
// 		productionPlantQty := item["production_plant_qty"].(float64)
// 		productionSubconQty := item["production_subcon_qty"].(float64)
// 		requireQty := item["require_qty"].(float64)
// 		urgentQty := item["urgent_qty"].(float64)
// 		confirmRequireQty := item["confirm_require_qty"].(float64)
// 		confirmRequireDate := item["confirm_require_date"].(time.Time)
// 		confirmUrgentQty := item["confirm_urgent_qty"].(float64)
// 		confirmUrgentDate := item["confirm_urgent_date"].(time.Time)
// 		leadTime := int64(item["delivery_lead_time"].(float64))

// 		jitDailyKey := fmt.Sprintf(`%s|%s|%s`, planDateStr, materialCode, lineCode)

// 		newJit := JitLine{
// 			id:                  id,
// 			PlanId:              planId,
// 			DailyDate:           planDate,
// 			MaterialCode:        materialCode,
// 			LineCode:            lineCode,
// 			ProductionQty:       productionQty,
// 			ProductionPlantQty:  productionPlantQty,
// 			ProductionSubconQty: productionSubconQty,
// 			RequireQty:          requireQty,
// 			UrgenQty:            urgentQty,
// 			ConfirmRequireQty:   confirmRequireQty,
// 			ConfirmUrgentQty:    confirmUrgentQty,
// 			ConfirmRequireDate:  &confirmRequireDate,
// 			ConfirmUrgentDate:   &confirmUrgentDate,
// 			LeadTime:            leadTime,
// 			RefReuqestID:        &refRequestId,
// 		}

// 		jitDailyMap[jitDailyKey] = append(jitDailyMap[jitDailyKey], newJit)
// 	}

// 	return jitDailyMap, nil
// }

// func MergeJitDaily(jitLineMap map[string][]JitLine, jitLineDBMap map[string][]JitLine) (map[string][]JitLine, error) {
// 	for jitLineKey, jitLines := range jitLineMap {
// 		for jitLineDBKey, jitLineDB := range jitLineDBMap {
// 			if jitLineKey == jitLineDBKey {
// 				sumConfirmRequireQty := 0.0
// 				sumConfirmUrgentQty := 0.0
// 				var maxConfirmRequireDate *time.Time
// 				var maxConfirmUrgentDate *time.Time

// 				for _, jitLineDB := range jitLineDB {
// 					if jitLineDB.ConfirmRequireDate != nil {
// 						if maxConfirmRequireDate == nil || maxConfirmRequireDate.Before(*jitLineDB.ConfirmRequireDate) {
// 							maxConfirmRequireDate = jitLineDB.ConfirmRequireDate
// 						}
// 					}

// 					if jitLineDB.ConfirmUrgentDate != nil {
// 						if maxConfirmUrgentDate == nil || maxConfirmUrgentDate.Before(*jitLineDB.ConfirmUrgentDate) {
// 							maxConfirmUrgentDate = jitLineDB.ConfirmUrgentDate
// 						}
// 					}

// 					sumConfirmRequireQty += jitLineDB.ConfirmRequireQty
// 					sumConfirmUrgentQty += jitLineDB.ConfirmUrgentQty
// 				}

// 				for i, jitLine := range jitLines {
// 					if i+1 == len(jitLines) {
// 						jitLineMap[jitLineKey][i].ConfirmRequireQty += sumConfirmRequireQty
// 						jitLineMap[jitLineKey][i].ConfirmUrgentQty += sumConfirmUrgentQty

// 						if maxConfirmRequireDate != nil && (jitLine.ConfirmRequireDate == nil || jitLine.ConfirmRequireDate.Before(*maxConfirmRequireDate)) {
// 							jitLineMap[jitLineKey][i].ConfirmRequireDate = maxConfirmRequireDate
// 						}

// 						if maxConfirmUrgentDate != nil && (jitLine.ConfirmUrgentDate == nil || jitLine.ConfirmUrgentDate.Before(*maxConfirmUrgentDate)) {
// 							jitLineMap[jitLineKey][i].ConfirmUrgentDate = maxConfirmUrgentDate
// 						}
// 					}
// 				}
// 			}
// 		}
// 	}

// 	for jitLineDBKey, jitLineDBs := range jitLineDBMap {
// 		if _, exist := jitLineMap[jitLineDBKey]; !exist {
// 			for _, jitLineDB := range jitLineDBs {
// 				newJit := JitLine{
// 					id:                  0,
// 					PlanId:              0,
// 					DailyDate:           jitLineDB.DailyDate,
// 					MaterialCode:        jitLineDB.MaterialCode,
// 					LineCode:            jitLineDB.LineCode,
// 					ProductionQty:       0,
// 					ProductionPlantQty:  0,
// 					ProductionSubconQty: 0,
// 					RequireQty:          0,
// 					UrgenQty:            0,
// 					ConfirmRequireQty:   jitLineDB.ConfirmRequireQty,
// 					ConfirmUrgentQty:    jitLineDB.ConfirmUrgentQty,
// 					ConfirmRequireDate:  jitLineDB.ConfirmRequireDate,
// 					ConfirmUrgentDate:   jitLineDB.ConfirmUrgentDate,
// 					LeadTime:            jitLineDB.LeadTime,
// 					RefReuqestID:        nil,
// 				}

// 				jitLineMap[jitLineDBKey] = append(jitLineMap[jitLineDBKey], newJit)
// 			}
// 		}
// 	}

// 	rowsIdCount := int64(1)
// 	for _, jitLines := range jitLineMap {
// 		for i := range jitLines {
// 			jitLines[i].id = rowsIdCount
// 			rowsIdCount++
// 		}
// 	}

// 	return jitLineMap, nil
// }

// func CalculateEstimate(jitLineMap map[string][]JitLine, matStock []MaterialStock, startCal time.Time) ([]JitMaterial, map[string][]JitDate, []string, []string, error) {
// 	jitMats := []JitMaterial{}
// 	matStockMap := map[string]MaterialStock{}
// 	confirmJitDateMap := map[string][]JitDate{}
// 	lineCodes := []string{}
// 	checkLineCodeMap := map[string]bool{}
// 	materialCodes := []string{}
// 	checkMaterialCodeMap := map[string]bool{}

// 	//map stock
// 	for _, item := range matStock {
// 		materialCode := item.MaterialCode
// 		key := materialCode
// 		matStockMap[key] = item
// 	}

// 	//convert and prepare data
// 	for _, jitLines := range jitLineMap {
// 		for _, jd := range jitLines {
// 			materialCode := jd.MaterialCode
// 			planDate := jd.DailyDate
// 			planDateTrucn := planDate.Truncate(24 * time.Hour)
// 			id := jd.id
// 			refReqId := jd.RefReuqestID
// 			lineCode := jd.LineCode
// 			productQty := jd.ProductionQty
// 			productPlantQty := jd.ProductionPlantQty
// 			productSubconQty := jd.ProductionSubconQty
// 			requireQty := jd.RequireQty
// 			urgentQty := jd.UrgenQty
// 			confirmRequireQty := jd.ConfirmRequireQty
// 			confirmRequireDate := jd.ConfirmRequireDate
// 			confirmUrgentQty := jd.ConfirmUrgentQty
// 			confirmUrgentDate := jd.ConfirmUrgentDate
// 			leadTime := jd.LeadTime

// 			lineCodeKey := lineCode
// 			if _, exist := checkLineCodeMap[lineCodeKey]; !exist {
// 				lineCodes = append(lineCodes, lineCode)
// 				checkLineCodeMap[lineCodeKey] = true
// 			}

// 			materialCodeKey := materialCode
// 			if _, exist := checkMaterialCodeMap[materialCodeKey]; !exist {
// 				materialCodes = append(materialCodes, materialCode)
// 				checkMaterialCodeMap[materialCodeKey] = true
// 			}

// 			matStock, exist := matStockMap[materialCode]
// 			if !exist {
// 				matStock = MaterialStock{}
// 			}

// 			newJitLine := JitLine{
// 				id:                  id,
// 				RefReuqestID:        refReqId,
// 				MaterialCode:        materialCode,
// 				DailyDate:           planDate,
// 				LineCode:            lineCode,
// 				ProductionQty:       productQty,
// 				ProductionPlantQty:  productPlantQty,
// 				ProductionSubconQty: productSubconQty,
// 				RequireQty:          requireQty,
// 				UrgenQty:            urgentQty,
// 				ConfirmRequireQty:   confirmRequireQty,
// 				ConfirmUrgentQty:    confirmUrgentQty,
// 				ConfirmRequireDate:  confirmRequireDate,
// 				ConfirmUrgentDate:   confirmUrgentDate,
// 			}

// 			newJitDate := JitDate{
// 				Date:               planDateTrucn,
// 				ProductionQty:      productQty,
// 				RequireQty:         requireQty,
// 				UrgentQty:          urgentQty,
// 				ConfirmQty:         confirmRequireQty + confirmUrgentQty,
// 				ConfirmRequireQty:  confirmRequireQty,
// 				ConfirmUrgentQty:   confirmUrgentQty,
// 				ConfirmRequireDate: confirmRequireDate,
// 				ConfirmUrgentDate:  confirmUrgentDate,
// 				Lines:              []JitLine{newJitLine},
// 			}

// 			newJitMat := JitMaterial{
// 				MaterialCode: materialCode,
// 				Stock:        matStock,
// 				LeadTime:     leadTime,
// 				JitDates:     []JitDate{newJitDate},
// 			}

// 			if confirmRequireDate != nil || confirmUrgentDate != nil {
// 				key := planDateTrucn.Format("2006-01-02")
// 				confirmJitDateMap[key] = append(confirmJitDateMap[key], newJitDate)
// 			}

// 			isFoundMat := false

// 			for cJitMat, jitMat := range jitMats {
// 				if jitMat.MaterialCode == materialCode {
// 					isFoundDate := false

// 					for cJitDate, jitDate := range jitMat.JitDates {
// 						if jitDate.Date.Equal(planDate) {

// 							jitMats[cJitMat].JitDates[cJitDate].ProductionQty += productQty
// 							jitMats[cJitMat].JitDates[cJitDate].RequireQty += requireQty
// 							jitMats[cJitMat].JitDates[cJitDate].UrgentQty += urgentQty
// 							jitMats[cJitMat].JitDates[cJitDate].ConfirmQty += confirmRequireQty + confirmUrgentQty
// 							jitMats[cJitMat].JitDates[cJitDate].ConfirmRequireQty += confirmRequireQty
// 							jitMats[cJitMat].JitDates[cJitDate].ConfirmUrgentQty += confirmUrgentQty

// 							if confirmRequireDate != nil {
// 								if jitMats[cJitMat].JitDates[cJitDate].ConfirmRequireDate == nil || jitMats[cJitMat].JitDates[cJitDate].ConfirmRequireDate.After(*confirmRequireDate) {
// 									jitMats[cJitMat].JitDates[cJitDate].ConfirmRequireDate = confirmRequireDate
// 								}
// 							}

// 							if confirmUrgentDate != nil {
// 								if jitMats[cJitMat].JitDates[cJitDate].ConfirmUrgentDate == nil || jitMats[cJitMat].JitDates[cJitDate].ConfirmUrgentDate.After(*confirmUrgentDate) {
// 									jitMats[cJitMat].JitDates[cJitDate].ConfirmUrgentDate = confirmUrgentDate
// 								}
// 							}

// 							jitMats[cJitMat].JitDates[cJitDate].Lines = append(jitMats[cJitMat].JitDates[cJitDate].Lines, newJitLine)

// 							isFoundDate = true
// 							break
// 						}
// 					}

// 					if !isFoundDate {
// 						jitMats[cJitMat].JitDates = append(jitMats[cJitMat].JitDates, newJitDate)
// 					}

// 					isFoundMat = true
// 					break
// 				}
// 			}

// 			if !isFoundMat {
// 				jitMats = append(jitMats, newJitMat)
// 			}
// 		}
// 	}

// 	for i := range jitMats {
// 		sort.Slice(jitMats[i].JitDates, func(a, b int) bool {
// 			return jitMats[i].JitDates[a].Date.Before(jitMats[i].JitDates[b].Date)
// 		})

// 		for j := range jitMats[i].JitDates {
// 			sort.Slice(jitMats[i].JitDates[j].Lines, func(a, b int) bool {
// 				if jitMats[i].JitDates[j].Lines[a].DailyDate.Equal(jitMats[i].JitDates[j].Lines[b].DailyDate) {
// 					return jitMats[i].JitDates[j].Lines[a].LineCode < jitMats[i].JitDates[j].Lines[b].LineCode
// 				}
// 				return jitMats[i].JitDates[j].Lines[a].DailyDate.Before(jitMats[i].JitDates[j].Lines[b].DailyDate)
// 			})
// 		}
// 	}

// 	//Cal update prod and estimate
// 	for cMat, jitMat := range jitMats {
// 		for cDate, jitDate := range jitMat.JitDates {
// 			isStockPlant := true

// 			if cDate == 0 {
// 				jitMats[cMat].JitDates[cDate].BeginStock = jitMat.Stock.StockPlantQty + jitMat.Stock.StockSubconQty

// 				if jitMat.Stock.StockPlantQty == 0 && jitMat.Stock.StockSubconQty > 0 {
// 					isStockPlant = false
// 				}

// 				jitMats[cMat].JitDates[cDate].PlantSock = jitMat.Stock.StockPlantQty
// 				jitMats[cMat].JitDates[cDate].SubconStock = jitMat.Stock.StockSubconQty
// 			} else {
// 				jitMats[cMat].JitDates[cDate].BeginStock = jitMats[cMat].JitDates[cDate-1].EstimateStock

// 				if isStockPlant {
// 					jitMats[cMat].JitDates[cDate].PlantSock = jitMats[cMat].JitDates[cDate].BeginStock
// 				} else {
// 					jitMats[cMat].JitDates[cDate].SubconStock = jitMats[cMat].JitDates[cDate].BeginStock
// 				}
// 			}

// 			//Check create require or urgent
// 			if jitDate.BeginStock-jitDate.ProductionQty < 0 {
// 				isUrgent := false
// 				leadTime := jitMat.LeadTime
// 				requireQty := math.Abs(jitDate.BeginStock - jitDate.ProductionQty)
// 				currentDateCount := int64(cDate)
// 				startUpdateData := currentDateCount - leadTime

// 				if startUpdateData < 0 {
// 					startUpdateData = currentDateCount
// 					isUrgent = true
// 				}

// 				for current := startUpdateData; current <= currentDateCount; current++ {
// 					if current == startUpdateData {
// 						if isUrgent {
// 							jitMats[cMat].JitDates[current].UrgentQty += requireQty
// 						} else {
// 							jitMats[cMat].JitDates[current].RequireQty += requireQty
// 						}

// 						remainRequireQty := requireQty
// 						for cRequireJitLine, requireJitLine := range jitMats[cMat].JitDates[current].Lines {
// 							useRequireQty := 0.0

// 							//Check productionQty
// 							for _, pruductionJitLine := range jitDate.Lines { //todo check หากว่ามาหลายๆ อันแบบไม่ใช่ line เดียวกันจะลงไม่ถูก ต้องแยกตาม plan
// 								if pruductionJitLine.LineCode == requireJitLine.LineCode && pruductionJitLine.RequireQty > 0 {
// 									useRequireQty = pruductionJitLine.RequireQty
// 									break
// 								}
// 							}

// 							if cRequireJitLine+1 == len(jitMats[cMat].JitDates[current].Lines) || useRequireQty > remainRequireQty {
// 								useRequireQty = remainRequireQty
// 							}

// 							if isUrgent {
// 								jitMats[cMat].JitDates[current].Lines[cRequireJitLine].UrgenQty = useRequireQty
// 							} else {
// 								jitMats[cMat].JitDates[current].Lines[cRequireJitLine].RequireQty = useRequireQty
// 							}

// 							remainRequireQty -= useRequireQty

// 							if remainRequireQty <= 0 {
// 								break
// 							}
// 						}
// 					} else {
// 						jitMats[cMat].JitDates[current].BeginStock = jitMats[cMat].JitDates[current-1].EstimateStock
// 					}

// 					estimateStock := jitMats[cMat].JitDates[current].BeginStock - jitMats[cMat].JitDates[current].ProductionQty + jitMats[cMat].JitDates[current].RequireQty + jitMats[cMat].JitDates[current].UrgentQty
// 					jitMats[cMat].JitDates[current].EstimateStock = estimateStock
// 				}

// 				continue
// 			}

// 			estimateStock := jitMats[cMat].JitDates[cDate].BeginStock - jitMats[cMat].JitDates[cDate].ProductionQty + jitMats[cMat].JitDates[cDate].RequireQty + jitMats[cMat].JitDates[cDate].UrgentQty
// 			jitMats[cMat].JitDates[cDate].EstimateStock = estimateStock
// 		}
// 	}

// 	return jitMats, confirmJitDateMap, materialCodes, lineCodes, nil
// }

// func CalculateActual(jitMats []JitMaterial, confirmJitDateMap map[string][]JitDate) ([]JitMaterial, error) {
// 	for cJitMat, jitMat := range jitMats {
// 		for cJitDate, jitDate := range jitMat.JitDates {
// 			diff := (jitDate.RequireQty + jitDate.UrgentQty) - (jitDate.ConfirmRequireQty + jitDate.ConfirmUrgentQty)
// 			beginStock := jitDate.BeginStock
// 			confirmQty := 0.0
// 			productionQty := jitDate.ProductionQty
// 			planDate := jitDate.Date
// 			planDateStr := planDate.Format("2006-01-02")

// 			key := planDateStr
// 			if confirmJit, exist := confirmJitDateMap[key]; exist {
// 				for _, item := range confirmJit {
// 					confirmQty += item.ConfirmQty
// 				}
// 			}

// 			if cJitDate != 0 {
// 				beginStock = jitMats[cJitMat].JitDates[cJitDate-1].EndStock
// 			}

// 			endStock := beginStock - productionQty + confirmQty

// 			jitMats[cJitMat].JitDates[cJitDate].Diff = diff
// 			jitMats[cJitMat].JitDates[cJitDate].EndStock = endStock
// 		}
// 	}

// 	return jitMats, nil
// }

// func GetLineMap(sqlx *sqlx.DB, condition []string) (map[string]Line, error) {
// 	rsMap := map[string]Line{}

// 	query := fmt.Sprintf(`
// 		select jlh.line_header_id as line_id
// 			,  jlh.line_header_name as line_code
// 		from jit_line_headers jlh
// 		where jlh.line_header_name in ('%s')
// 	`, strings.Join(condition, `','`))
// 	rows, err := db.ExecuteQuery(sqlx, query)
// 	if err != nil {
// 		return nil, err
// 	}

// 	for _, item := range rows {
// 		lineId := item["line_id"].(int64)
// 		lineCode := item["line_code"].(string)

// 		key := lineCode
// 		rsMap[key] = Line{
// 			LineId:   lineId,
// 			LineCode: lineCode,
// 		}
// 	}

// 	return rsMap, nil
// }

// func GetMatrialMap(sqlx *sqlx.DB, condition []string) (map[string]Material, error) {
// 	rsMap := map[string]Material{}

// 	query := fmt.Sprintf(`
// 		select m.material_id ,m.material_code , m.supplier_id
// 		,  coalesce(s.supplier_code , '') as supplier_code
// 		from materials m
// 		left join suppliers s on m.supplier_id  = s.supplier_id
// 		where m.material_code in ('%s')
// 	`, strings.Join(condition, `','`))
// 	rows, err := db.ExecuteQuery(sqlx, query)
// 	if err != nil {
// 		return nil, err
// 	}

// 	for _, item := range rows {
// 		materialId := item["material_id"].(int64)
// 		materialCode := item["material_code"].(string)
// 		supplierId := item["supplier_id"].(int64)
// 		supplierCode := item["supplier_code"].(string)

// 		key := materialCode
// 		rsMap[key] = Material{
// 			MaterialId:   materialId,
// 			MaterialCode: materialCode,
// 			SupplierId:   supplierId,
// 			SupplierCode: supplierCode,
// 		}
// 	}

// 	return rsMap, nil
// }

// func ConvertToJitDailyDB(jitMats []JitMaterial, lineMap map[string]Line, materialMap map[string]Material) ([]JitDaily, error) {
// 	jitDailys := []JitDaily{}

// 	for _, jitMat := range jitMats {
// 		for _, jitDate := range jitMat.JitDates {
// 			for _, jitLine := range jitDate.Lines {
// 				lineCode := jitLine.LineCode
// 				materialCode := jitLine.MaterialCode
// 				lineId := int64(0)
// 				materialId := int64(0)
// 				supplierId := int64(0)

// 				lineKey := lineCode
// 				if line, exist := lineMap[lineKey]; exist {
// 					lineId = line.LineId
// 				}

// 				materialKey := materialCode
// 				if material, exist := materialMap[materialKey]; exist {
// 					materialId = material.MaterialId
// 					supplierId = material.SupplierId
// 				}

// 				jitDaily := JitDaily{
// 					JitDailyID:         jitLine.id,
// 					JitDailyPlanID:     jitLine.PlanId,
// 					MaterialID:         materialId,
// 					LineID:             lineId,
// 					SupplierID:         supplierId,
// 					DailyDate:          jitDate.Date,
// 					BeginStock:         jitDate.BeginStock,
// 					PlantStock:         jitDate.PlantSock,
// 					SubconStock:        jitDate.SubconStock,
// 					EndOfStock:         jitDate.EstimateStock,
// 					ProductQty:         jitLine.ProductionQty,
// 					PlantQty:           jitLine.ProductionPlantQty,
// 					SubconQty:          jitLine.ProductionSubconQty,
// 					RequiredQty:        jitLine.RequireQty,
// 					UrgentQty:          jitLine.UrgenQty,
// 					ConfQty:            jitLine.ConfirmRequireQty,
// 					ConfDate:           *jitLine.ConfirmRequireDate,
// 					ConfUrgentQty:      jitLine.ConfirmUrgentQty,
// 					ConfUrgentDate:     *jitLine.ConfirmUrgentDate,
// 					OriginalJitDailyID: *jitLine.RefReuqestID,
// 					IsDeleted:          false,
// 					IsGenerate:         false,
// 					CreatedDate:        time.Now(),
// 					UpdatedDate:        time.Now(),
// 				}

// 				jitDailys = append(jitDailys, jitDaily)
// 			}
// 		}
// 	}

// 	return jitDailys, nil
// }

// // todo ส่ง jit_process มาอะำ
// func CreateJitDaily(gormx *gorm.DB, jitDailys []JitDaily, startDate time.Time) error {
// 	mats := []int64{}
// 	matCheck := map[int64]bool{}
// 	for _, item := range jitDailys {
// 		materialId := item.MaterialID
// 		if _, exist := matCheck[materialId]; !exist {
// 			mats = append(mats, materialId)

// 			matCheck[materialId] = true
// 		}
// 	}

// 	tx := gormx.Begin()

// 	var maxID int64
// 	if err := tx.Raw("SELECT COALESCE(MAX(jit_daily_id), 0) AS max_id FROM jit_daily").Scan(&maxID).Error; err != nil {
// 		tx.Rollback()
// 		return fmt.Errorf("failed to get max id: %w", err)
// 	}

// 	for i := range jitDailys {
// 		maxID++
// 		jitDailys[i].JitDailyID = maxID
// 	}

// 	if err := tx.Where("daily_date >= ? AND material_id IN ?", startDate, mats).Delete(&JitDaily{}).Error; err != nil {
// 		tx.Rollback()
// 		return fmt.Errorf("failed to delete existing records: %w", err)
// 	}

// 	if err := tx.Create(&jitDailys).Error; err != nil {
// 		tx.Rollback()
// 		return fmt.Errorf("failed to insert new records: %w", err)
// 	}

// 	if err := tx.Commit().Error; err != nil {
// 		return fmt.Errorf("failed to commit transaction: %w", err)
// 	}
// 	return nil
// }
