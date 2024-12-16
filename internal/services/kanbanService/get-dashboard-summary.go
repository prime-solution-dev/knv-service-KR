package kanbanService

import (
	"encoding/json"
	"errors"
	"fmt"
	"jnv-jit/internal/db"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type GetDashboardSummaryRequest struct {
	Suppliers []string `json:"suppliers"`
	WeekData  int      `json:"week_data"`
	Materials []string `json:"materials"`
}

type CalSummaryKPI struct {
	RequestDate     time.Time
	RequestType     string
	SupplierCode    string
	MaterialID      string
	Sku             string
	RequestQty      int
	ConfirmQty      int
	ActualQty       int
	PercentDiffQty  int
	ActualQtyKPI    bool
	EstimateDate    *time.Time
	ConfirmDate     *time.Time
	ActualDate      *time.Time
	DeliveryDateKPI bool
	SummaryKPI      bool
	LeadTime        float64
}

type StockDay struct {
	Date       time.Time
	MaterialID string
	Qty        int
}

type Week struct {
	WeekNumber  int
	StartOfWeek time.Time
	EndOfWeek   time.Time
}

type GetDashboardSummaryResponse struct {
	Summarys []KanbanDashboardSummaryKpiData `json:"summarys"`
	Kpis     KanbanDashboardKpiData          `json:"kpis"`
}

type KanbanDashboardSummaryKpiData struct {
	Label          string `json:"label"`
	TotalRequest   int    `json:"totalRequest"`
	TotalPass      int    `json:"totalPass"`
	KpiPassPercent int    `json:"kpiPassPercent"`
}

type KanbanDashboardKpiData struct {
	LabelList                             []string  `json:"labelList"`
	DeliveryOntimeList                    []int     `json:"deliveryOntimeList"`
	DeliveryEarlyList                     []int     `json:"deliveryEarlyList"`
	DeliveryLateList                      []int     `json:"deliveryLateList"`
	DeliverySum                           []int     `json:"deliverySum"`
	ActualQtyPassList                     []int     `json:"actualQtyPassList"`
	ActualQtyFailList                     []int     `json:"actualQtyFailList"`
	YellowSupplierDeliveryExpectList      []float64 `json:"yellowSupplierDeliveryExpectList"`
	RedSupplierDeliveryExpectList         []float64 `json:"redSupplierDeliveryExpectList"`
	YellowAvgSupplierDeliveryLeadtimeList []float64 `json:"yellowAvgSupplierDeliveryLeadtimeList"`
	RedAvgSupplierDeliveryLeadtimeList    []float64 `json:"redAvgSupplierDeliveryLeadtimeList"`
	TotalLeadtimeRed                      float64   `json:"total_leadtime_red"`
	TotalLeadtimeYellow                   float64   `json:"total_leadtime_yellow"`
	TotalDatediffRed                      float64   `json:"total_datediff_red"`
	TotalDatediffYellow                   float64   `json:"total_datediff_yellow"`
}

func GetDashboardSummary(c *gin.Context, jsonPayload string) (interface{}, error) {
	var req GetDashboardSummaryRequest
	var res GetDashboardSummaryResponse

	if err := json.Unmarshal([]byte(jsonPayload), &req); err != nil {
		return nil, errors.New("failed to unmarshal JSON into struct: " + err.Error())
	}

	sqlx, err := db.ConnectSqlx(`jit_portal`)
	if err != nil {
		return nil, err
	}
	defer sqlx.Close()

	if req.WeekData == 0 {
		return nil, fmt.Errorf(`missing weekdata`)
	}

	qCondSupplier := ``
	qCondMaterial := ``
	qCondWeekData := ``

	if len(req.Suppliers) > 0 {
		qCondSupplier = fmt.Sprintf(` and s.supplier_code in ('%s') `, strings.Join(req.Suppliers, `','`))
	}

	if len(req.Materials) > 0 {
		qCondMaterial = fmt.Sprintf(` and m.material_code in ('%s')`, strings.Join(req.Materials, `','`))
	}

	firstDate := GetEarliestMonday(time.Now(), req.WeekData).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02") //Todo change param end_date

	qCondWeekData = fmt.Sprintf(` 
									and (kp.yellow_date between '%s' and '%s' or  kp.yellow_date between '%s' and '%s')
								`, firstDate, endDate,
		firstDate, endDate)

	queryKanban := fmt.Sprintf(`
		select kp.kanban_progress_id 
			, case when kp.yellow_date is not null then 'YELLOW' else 'RED' end request_type
			, case when kp.yellow_date is not null then kp.yellow_date else kp.red_date end request_date
			, kp.material_id 
			, case when kp.yellow_date is not null then coalesce(m.yellow1_day,0) else coalesce(m.red1_day,0) end delivery_leadtime
			, kp.qty as request_qty
			, kp.sup_conf_date as confirm_date
			, coalesce(kp.conf_qty,0) as confirm_qty
			, s.supplier_code
			, s.supplier_name 
			, m.material_code 
			, m.description as material_name
		from kanban_progress kp
		left join materials m on m.material_id = kp.material_id
		left join suppliers s on m.supplier_id = s.supplier_id 
		where 1=1
		and	(kp.yellow_date >= '2024-11-1' or kp.red_date >= '2024-11-1')
		%s
		%s
		%s
	`, qCondSupplier, qCondMaterial, qCondWeekData)
	println(queryKanban)
	rowsKanban, err := db.ExecuteQuery(sqlx, queryKanban)
	if err != nil {
		return nil, err
	}

	kanbanDateMap := make(map[string][]CalSummaryKPI)
	materialIdListMap := make(map[string]bool)
	materialStartDateMap := make(map[string]StockDay)
	var materialIdList []string

	for _, item := range rowsKanban {
		requestDate := item["request_date"].(time.Time).Truncate(24 * time.Hour)
		materialID := fmt.Sprintf("%d", item["material_id"].(int64))
		confirmDate := item["confirm_date"]
		leadtime := int(item["delivery_leadtime"].(float64))
		confirmQty := 0
		requestQty := int(item["request_qty"].(float64))

		var confirmDatePtr *time.Time
		if confirmDate != nil {
			dateValue, ok := confirmDate.(time.Time)
			if ok {
				truncatedDate := dateValue.Truncate(24 * time.Hour)
				confirmDatePtr = &truncatedDate
			}
		}

		var estimateDate *time.Time
		actualQtyKPI := false

		if confirmDate != nil {
			confirmQty = int(item["confirm_qty"].(float64))

			if confirmQty > 0 && requestQty > 0 {
				percentDiff := ((confirmQty - requestQty) / requestQty) * 100
				if percentDiff <= 5 && percentDiff >= -5 {
					actualQtyKPI = true
				}
			}

			estimateDateAdd := requestDate.AddDate(0, 0, leadtime)
			estimateDate = &estimateDateAdd

			if item, exist := materialStartDateMap[materialID]; exist {
				if item.Date.After(*confirmDatePtr) {
					materialStartDateMap[materialID] = StockDay{
						MaterialID: materialID,
						Date:       *confirmDatePtr,
					}
				}
			} else {
				materialStartDateMap[materialID] = StockDay{
					MaterialID: materialID,
					Date:       *confirmDatePtr,
				}
			}
		}

		kanbanKey := materialID
		kanbanDateMap[kanbanKey] = append(kanbanDateMap[kanbanKey], CalSummaryKPI{
			RequestDate:     requestDate,
			RequestType:     item["request_type"].(string),
			SupplierCode:    item["supplier_code"].(string),
			MaterialID:      materialID,
			Sku:             item["material_code"].(string),
			RequestQty:      int(item["request_qty"].(float64)),
			ConfirmQty:      int(item["confirm_qty"].(float64)),
			ActualQty:       0,
			PercentDiffQty:  0,
			ActualQtyKPI:    actualQtyKPI,
			EstimateDate:    estimateDate,
			ConfirmDate:     confirmDatePtr,
			ActualDate:      nil,
			DeliveryDateKPI: false,
			SummaryKPI:      false,
			LeadTime:        item["delivery_leadtime"].(float64),
		})

		materialIdKey := materialID 
		if _, exist := materialIdListMap[materialIdKey]; !exist {
			materialIdList = append(materialIdList, materialID)
			materialIdListMap[materialIdKey] = true
		}
	}

	var condStockDayMaterialDay []string
	for _, item := range materialStartDateMap {
		value := fmt.Sprintf(` (is2.material_id = '%s' and is2."date" >= '%s') `, item.MaterialID, item.Date.Format("2006-01-02"))
		condStockDayMaterialDay = append(condStockDayMaterialDay, value)
	}

	if len(condStockDayMaterialDay) == 0 || len(materialIdList) == 0 {
		return nil, fmt.Errorf(`not fount data`)
	}

	queryStockDay := fmt.Sprintf(`
		select is2.material_id, is2.current_gr as qty, is2."date" 
		from inventory_snapshots is2 
		where is2.current_gr is not null and coalesce (is2.current_gr, 0 ) <> 0
		and is2.material_id in ('%s')
		and (
				%s
			)
	`, strings.Join(materialIdList, `','`),
		strings.Join(condStockDayMaterialDay, ` or `))
	println(queryStockDay)
	rowsStockDay, err := db.ExecuteQuery(sqlx, queryStockDay)
	if err != nil {
		return nil, err
	}

	StockDayDb := make(map[string][]StockDay)
	for _, item := range rowsStockDay {
		materailId := fmt.Sprintf("%d", item["material_id"].(int64))
		date := item["date"].(time.Time)
		qty := int(item["qty"].(float64))

		stockDayKey := materailId
		StockDayDb[stockDayKey] = append(StockDayDb[stockDayKey], StockDay{
			Date:       date,
			MaterialID: materailId,
			Qty:        qty,
		})
	}

	calSum, err := calSummaryKPI(kanbanDateMap, StockDayDb)
	if err != nil {
		return nil, err
	}

	var errRes error

	if len(req.Suppliers) == 1 {
		res, errRes = summaryByWeek(calSum, req.WeekData)
	} else {
		res, errRes = summaryBySupplier(calSum)
	}

	if errRes != nil {
		return nil, errRes
	}

	return res, nil
}

func calSummaryKPI(kanbanData map[string][]CalSummaryKPI, stockData map[string][]StockDay) (map[string][]CalSummaryKPI, error) {

	for kanbanKey, kanban := range kanbanData {
		sort.Slice(kanban, func(i, j int) bool { //Todo เหมือนต้องเรียงวันที่ก่อนค่อยเรียงตาม Red
			if kanban[i].RequestType == "RED" && kanban[j].RequestType != "RED" {
				return true
			}
			if kanban[i].RequestType != "RED" && kanban[j].RequestType == "RED" {
				return false
			}

			if kanban[i].ConfirmDate == nil && kanban[j].ConfirmDate != nil {
				return false
			}
			if kanban[i].ConfirmDate != nil && kanban[j].ConfirmDate == nil {
				return true
			}
			if kanban[i].ConfirmDate == nil && kanban[j].ConfirmDate == nil {
				return false
			}

			return kanban[i].ConfirmDate.Before(*kanban[j].ConfirmDate)
		})

		stockMat, existStock := stockData[kanbanKey]
		if !existStock || len(stockMat) == 0 {
			continue
		}

		sort.Slice(stockMat, func(i, j int) bool {
			return stockMat[i].Date.Before(stockMat[j].Date)
		})

		allowcateStock := make(map[string]bool)
		for cKanbanItem, kanbanItem := range kanban {
			if kanbanItem.ConfirmDate == nil {
				continue
			}

			kConfirmDate := kanbanItem.ConfirmDate.Truncate(24 * time.Hour)
			kConfirmDateStr := kConfirmDate.Format("2006-01-02")
			kRequestType := kanbanItem.RequestType
			kRequestMaterial := kanbanItem.MaterialID

			for _, stockItem := range stockMat {
				sDate := stockItem.Date.Truncate(24 * time.Hour)
				sQty := stockItem.Qty

				allowcateStockKey := fmt.Sprintf(`%s|%s|%s`, kConfirmDateStr, kRequestType, kRequestMaterial)
				if _, existAllowcate := allowcateStock[allowcateStockKey]; !existAllowcate && (kConfirmDate.Equal(sDate) || kConfirmDate.Before(sDate)) {

					kanbanData[kanbanKey][cKanbanItem].ActualDate = &sDate
					kanbanData[kanbanKey][cKanbanItem].ActualQty = sQty

					if kanbanData[kanbanKey][cKanbanItem].ActualDate != nil && kanbanData[kanbanKey][cKanbanItem].ConfirmDate != nil {
						if kanbanData[kanbanKey][cKanbanItem].ActualDate.Equal(*kanbanData[kanbanKey][cKanbanItem].ConfirmDate) {
							kanbanData[kanbanKey][cKanbanItem].DeliveryDateKPI = true
						}
					}

					if kanbanData[kanbanKey][cKanbanItem].ActualQtyKPI && kanbanData[kanbanKey][cKanbanItem].DeliveryDateKPI {
						kanbanData[kanbanKey][cKanbanItem].SummaryKPI = true
					}

					allowcateStock[allowcateStockKey] = true
					break
				}
			}
		}
	}

	return kanbanData, nil
}

func summaryBySupplier(datas map[string][]CalSummaryKPI) (GetDashboardSummaryResponse, error) {
	var res GetDashboardSummaryResponse
	summaryMap := make(map[string]KanbanDashboardSummaryKpiData)
	kpiMap := make(map[string]KanbanDashboardKpiData)
	leadtimeYellowMap := make(map[string]float64)
	leadtimeRedMap := make(map[string]float64)

	for _, sumKPI := range datas {
		for _, item := range sumKPI {
			supplierCode := item.SupplierCode
			requestType := item.RequestType
			key := supplierCode
			totalRequest := 0
			totalPassKpi := 0
			totalFailKpi := 0
			passKpiPercent := 0

			totalRequest++

			//Summary
			if item.SummaryKPI {
				totalPassKpi++
			} else {
				totalFailKpi++
			}

			if _, exist := summaryMap[key]; !exist {
				if totalPassKpi != 0 && totalRequest != 0 {
					passKpiPercent = int((float64(totalPassKpi) / float64(totalRequest)) * 100)
				}

				sumItem := KanbanDashboardSummaryKpiData{
					Label:          supplierCode,
					TotalRequest:   totalRequest,
					TotalPass:      totalPassKpi,
					KpiPassPercent: passKpiPercent,
				}

				summaryMap[key] = sumItem
			} else {
				sumItem := summaryMap[key]
				sumItem.TotalPass += totalPassKpi
				sumItem.TotalRequest += totalRequest

				if sumItem.TotalPass != 0 && sumItem.TotalRequest != 0 {
					sumItem.KpiPassPercent = int((float64(sumItem.TotalPass) / float64(sumItem.TotalRequest)) * 100)
				}

				summaryMap[key] = sumItem
			}

			//KPI
			deliveryOntimeList := 0
			deliveryEarlyList := 0
			deliveryLateList := 0
			deliverySum := totalRequest
			actualQtyPassList := 0
			actualQtyFailList := 0
			yellowSupplierDeliveryExpectList := 0.0
			redSupplierDeliveryExpectList := 0.0
			yellowAvgSupplierDeliveryLeadtimeList := 0.0
			redAvgSupplierDeliveryLeadtimeList := 0.0
			totalLeadtimeRed := 0.0
			totalLeadtimeYellow := 0.0
			totalDatediffRed := 0.0
			totalDatediffYellow := 0.0

			if item.RequestDate.Before(*item.ActualDate) {
				deliveryLateList++
			}

			if item.RequestDate.Equal(*item.ActualDate) {
				deliveryOntimeList++
			}

			if item.RequestDate.After(*item.ActualDate) {
				deliveryEarlyList++
			}

			if item.ActualQtyKPI {
				actualQtyPassList++
			} else {
				actualQtyFailList++
			}

			if requestType == `RED` {
				totalLeadtimeRed = item.LeadTime
				totalDatediffRed = item.ActualDate.Sub(item.RequestDate).Hours() / 24

				defaultCoutLeadtime := 0.0
				if _, exist := leadtimeRedMap[key]; !exist {
					defaultCoutLeadtime = leadtimeRedMap[key]
				}
				leadtimeRedMap[key] = defaultCoutLeadtime + 1

			} else {
				totalLeadtimeYellow = item.LeadTime
				totalDatediffYellow = item.ActualDate.Sub(item.RequestDate).Hours() / 24

				defaultCoutLeadtime := 0.0
				if _, exist := leadtimeYellowMap[key]; exist {
					defaultCoutLeadtime = leadtimeYellowMap[key]
				}
				leadtimeYellowMap[key] = defaultCoutLeadtime + 1
			}

			if _, exist := kpiMap[key]; !exist {
				kpiItem := KanbanDashboardKpiData{
					LabelList:                             []string{supplierCode},
					DeliveryOntimeList:                    []int{deliveryOntimeList},
					DeliveryEarlyList:                     []int{deliveryEarlyList},
					DeliveryLateList:                      []int{deliveryLateList},
					DeliverySum:                           []int{deliverySum},
					ActualQtyPassList:                     []int{actualQtyPassList},
					ActualQtyFailList:                     []int{actualQtyFailList},
					YellowSupplierDeliveryExpectList:      []float64{yellowSupplierDeliveryExpectList},
					RedSupplierDeliveryExpectList:         []float64{redSupplierDeliveryExpectList},
					YellowAvgSupplierDeliveryLeadtimeList: []float64{yellowAvgSupplierDeliveryLeadtimeList},
					RedAvgSupplierDeliveryLeadtimeList:    []float64{redAvgSupplierDeliveryLeadtimeList},
					TotalLeadtimeRed:                      totalLeadtimeRed,
					TotalLeadtimeYellow:                   totalLeadtimeYellow,
					TotalDatediffRed:                      totalDatediffRed,
					TotalDatediffYellow:                   totalDatediffYellow,
				}

				kpiMap[key] = kpiItem
			} else {
				kpiItem := kpiMap[key]

				kpiItem.TotalLeadtimeRed += totalLeadtimeRed
				kpiItem.TotalLeadtimeYellow += totalLeadtimeYellow
				kpiItem.TotalDatediffRed += totalDatediffRed
				kpiItem.TotalDatediffYellow += totalDatediffYellow

				if kpiItem.TotalLeadtimeRed > 0 {
					redSupplierDeliveryExpectList = kpiItem.TotalLeadtimeRed / leadtimeRedMap[key]
				}

				if kpiItem.TotalLeadtimeYellow > 0 {
					yellowSupplierDeliveryExpectList = kpiItem.TotalLeadtimeYellow / leadtimeYellowMap[key]
				}

				if kpiItem.TotalDatediffRed > 0 {
					redAvgSupplierDeliveryLeadtimeList = kpiItem.TotalDatediffRed / leadtimeRedMap[key]
				}

				if kpiItem.TotalDatediffYellow > 0 {
					yellowAvgSupplierDeliveryLeadtimeList = kpiItem.TotalDatediffYellow / leadtimeYellowMap[key]
				}

				kpiItem.DeliveryOntimeList[0] = kpiItem.DeliveryOntimeList[0] + deliveryOntimeList
				kpiItem.DeliveryEarlyList[0] = kpiItem.DeliveryEarlyList[0] + deliveryEarlyList
				kpiItem.DeliveryLateList[0] = kpiItem.DeliveryLateList[0] + deliveryLateList
				kpiItem.DeliverySum[0] = kpiItem.DeliverySum[0] + deliverySum
				kpiItem.ActualQtyPassList[0] = kpiItem.ActualQtyPassList[0] + actualQtyPassList
				kpiItem.ActualQtyFailList[0] = kpiItem.ActualQtyFailList[0] + actualQtyFailList
				kpiItem.YellowSupplierDeliveryExpectList[0] = kpiItem.YellowSupplierDeliveryExpectList[0] + yellowSupplierDeliveryExpectList
				kpiItem.RedSupplierDeliveryExpectList[0] = kpiItem.RedSupplierDeliveryExpectList[0] + redSupplierDeliveryExpectList
				kpiItem.YellowAvgSupplierDeliveryLeadtimeList[0] = kpiItem.YellowAvgSupplierDeliveryLeadtimeList[0] + yellowAvgSupplierDeliveryLeadtimeList
				kpiItem.RedAvgSupplierDeliveryLeadtimeList[0] = kpiItem.RedAvgSupplierDeliveryLeadtimeList[0] + redAvgSupplierDeliveryLeadtimeList

				kpiMap[key] = kpiItem
			}

		}
	}

	for _, item := range summaryMap {
		res.Summarys = append(res.Summarys, item)
	}

	for _, item := range kpiMap {
		res.Kpis.LabelList = append(res.Kpis.LabelList, item.LabelList[0])
		res.Kpis.DeliveryOntimeList = append(res.Kpis.DeliveryOntimeList, item.DeliveryOntimeList[0])
		res.Kpis.DeliveryEarlyList = append(res.Kpis.DeliveryEarlyList, item.DeliveryEarlyList[0])
		res.Kpis.DeliveryLateList = append(res.Kpis.DeliveryLateList, item.DeliveryLateList[0])
		res.Kpis.DeliverySum = append(res.Kpis.DeliverySum, item.DeliverySum[0])
		res.Kpis.ActualQtyPassList = append(res.Kpis.ActualQtyPassList, item.ActualQtyPassList[0])
		res.Kpis.ActualQtyFailList = append(res.Kpis.ActualQtyFailList, item.ActualQtyFailList[0])
		res.Kpis.YellowSupplierDeliveryExpectList = append(res.Kpis.YellowSupplierDeliveryExpectList, item.YellowSupplierDeliveryExpectList[0])
		res.Kpis.RedSupplierDeliveryExpectList = append(res.Kpis.RedSupplierDeliveryExpectList, item.RedSupplierDeliveryExpectList[0])
		res.Kpis.YellowAvgSupplierDeliveryLeadtimeList = append(res.Kpis.YellowAvgSupplierDeliveryLeadtimeList, item.YellowAvgSupplierDeliveryLeadtimeList[0])
		res.Kpis.RedAvgSupplierDeliveryLeadtimeList = append(res.Kpis.RedAvgSupplierDeliveryLeadtimeList, item.RedAvgSupplierDeliveryLeadtimeList[0])
	}

	return res, nil
}

func summaryByWeek(datas map[string][]CalSummaryKPI, WeekData int) (GetDashboardSummaryResponse, error) {
	var res GetDashboardSummaryResponse
	weeks := GenerateBackWeeks(time.Now(), WeekData)
	summaryMap := make(map[string]KanbanDashboardSummaryKpiData)
	kpiMap := make(map[string]KanbanDashboardKpiData)
	leadtimeYellowMap := make(map[string]float64)
	leadtimeRedMap := make(map[string]float64)

	for _, item := range weeks {
		key := strconv.Itoa(item.WeekNumber)

		sumItem := KanbanDashboardSummaryKpiData{
			Label:          item.StartOfWeek.Format("02/01/2006"),
			TotalRequest:   0,
			TotalPass:      0,
			KpiPassPercent: 0,
		}

		kpiItem := KanbanDashboardKpiData{
			LabelList:                             []string{item.StartOfWeek.Format("02/01/2006")},
			DeliveryOntimeList:                    []int{0},
			DeliveryEarlyList:                     []int{0},
			DeliveryLateList:                      []int{0},
			DeliverySum:                           []int{0},
			ActualQtyPassList:                     []int{0},
			ActualQtyFailList:                     []int{0},
			YellowSupplierDeliveryExpectList:      []float64{0},
			RedSupplierDeliveryExpectList:         []float64{0},
			YellowAvgSupplierDeliveryLeadtimeList: []float64{0},
			RedAvgSupplierDeliveryLeadtimeList:    []float64{0},
			TotalLeadtimeRed:                      0,
			TotalLeadtimeYellow:                   0,
			TotalDatediffRed:                      0,
			TotalDatediffYellow:                   0,
		}

		summaryMap[key] = sumItem
		kpiMap[key] = kpiItem
	}

	for _, sumKPI := range datas {
		for _, item := range sumKPI {
			_, week := item.RequestDate.ISOWeek()
			weekStr := strconv.Itoa(week)
			key := weekStr
			requestType := item.RequestType

			//Summary
			totalRequest := 0
			totalPassKpi := 0
			totalFailKpi := 0
			passKpiPercent := 0

			totalRequest++

			if item.SummaryKPI {
				totalPassKpi++
			} else {
				totalFailKpi++
			}

			if _, exist := summaryMap[key]; exist {
				sumItem := summaryMap[key]
				sumItem.TotalRequest += totalRequest
				sumItem.KpiPassPercent += totalPassKpi
				sumItem.KpiPassPercent += passKpiPercent

				if sumItem.KpiPassPercent != 0 && sumItem.TotalRequest != 0 {
					sumItem.KpiPassPercent = int((float64(sumItem.KpiPassPercent) / float64(sumItem.TotalRequest)) * 100)
				}

				summaryMap[key] = sumItem
			}

			//KPI
			deliveryOntimeList := 0
			deliveryEarlyList := 0
			deliveryLateList := 0
			deliverySum := totalRequest
			actualQtyPassList := 0
			actualQtyFailList := 0
			yellowSupplierDeliveryExpectList := 0.0
			redSupplierDeliveryExpectList := 0.0
			yellowAvgSupplierDeliveryLeadtimeList := 0.0
			redAvgSupplierDeliveryLeadtimeList := 0.0
			totalLeadtimeRed := 0.0
			totalLeadtimeYellow := 0.0
			totalDatediffRed := 0.0
			totalDatediffYellow := 0.0

			if item.RequestDate.Before(*item.ActualDate) {
				deliveryLateList++
			}

			if item.RequestDate.Equal(*item.ActualDate) {
				deliveryOntimeList++
			}

			if item.RequestDate.After(*item.ActualDate) {
				deliveryEarlyList++
			}

			if item.ActualQtyKPI {
				actualQtyPassList++
			} else {
				actualQtyFailList++
			}

			if requestType == `RED` {
				totalLeadtimeRed = item.LeadTime
				totalDatediffRed = item.ActualDate.Sub(item.RequestDate).Hours() / 24

				defaultCoutLeadtime := 0.0
				if _, exist := leadtimeRedMap[key]; !exist {
					defaultCoutLeadtime = leadtimeRedMap[key]
				}
				leadtimeRedMap[key] = defaultCoutLeadtime + 1

			} else {
				totalLeadtimeYellow = item.LeadTime
				totalDatediffYellow = item.ActualDate.Sub(item.RequestDate).Hours() / 24

				defaultCoutLeadtime := 0.0
				if _, exist := leadtimeYellowMap[key]; exist {
					defaultCoutLeadtime = leadtimeYellowMap[key]
				}
				leadtimeYellowMap[key] = defaultCoutLeadtime + 1
			}

			if kpiItem, exist := kpiMap[key]; exist {
				kpiItem.TotalLeadtimeRed += totalLeadtimeRed
				kpiItem.TotalLeadtimeYellow += totalLeadtimeYellow
				kpiItem.TotalDatediffRed += totalDatediffRed
				kpiItem.TotalDatediffYellow += totalDatediffYellow

				if kpiItem.TotalLeadtimeRed > 0 {
					redSupplierDeliveryExpectList = kpiItem.TotalLeadtimeRed / leadtimeRedMap[key]
				}

				if kpiItem.TotalLeadtimeYellow > 0 {
					yellowSupplierDeliveryExpectList = kpiItem.TotalLeadtimeYellow / leadtimeYellowMap[key]
				}

				if kpiItem.TotalDatediffRed > 0 {
					redAvgSupplierDeliveryLeadtimeList = kpiItem.TotalDatediffRed / leadtimeRedMap[key]
				}

				if kpiItem.TotalDatediffYellow > 0 {
					yellowAvgSupplierDeliveryLeadtimeList = kpiItem.TotalDatediffYellow / leadtimeYellowMap[key]
				}

				kpiItem.DeliveryOntimeList[0] = kpiItem.DeliveryOntimeList[0] + deliveryOntimeList
				kpiItem.DeliveryEarlyList[0] = kpiItem.DeliveryEarlyList[0] + deliveryEarlyList
				kpiItem.DeliveryLateList[0] = kpiItem.DeliveryLateList[0] + deliveryLateList
				kpiItem.DeliverySum[0] = kpiItem.DeliverySum[0] + deliverySum
				kpiItem.ActualQtyPassList[0] = kpiItem.ActualQtyPassList[0] + actualQtyPassList
				kpiItem.ActualQtyFailList[0] = kpiItem.ActualQtyFailList[0] + actualQtyFailList
				kpiItem.YellowSupplierDeliveryExpectList[0] = kpiItem.YellowSupplierDeliveryExpectList[0] + yellowSupplierDeliveryExpectList
				kpiItem.RedSupplierDeliveryExpectList[0] = kpiItem.RedSupplierDeliveryExpectList[0] + redSupplierDeliveryExpectList
				kpiItem.YellowAvgSupplierDeliveryLeadtimeList[0] = kpiItem.YellowAvgSupplierDeliveryLeadtimeList[0] + yellowAvgSupplierDeliveryLeadtimeList
				kpiItem.RedAvgSupplierDeliveryLeadtimeList[0] = kpiItem.RedAvgSupplierDeliveryLeadtimeList[0] + redAvgSupplierDeliveryLeadtimeList

				kpiMap[key] = kpiItem
			}
		}
	}

	for _, item := range summaryMap {
		res.Summarys = append(res.Summarys, item)
	}

	for _, item := range kpiMap {
		res.Kpis.LabelList = append(res.Kpis.LabelList, item.LabelList[0])
		res.Kpis.DeliveryOntimeList = append(res.Kpis.DeliveryOntimeList, item.DeliveryOntimeList[0])
		res.Kpis.DeliveryEarlyList = append(res.Kpis.DeliveryEarlyList, item.DeliveryEarlyList[0])
		res.Kpis.DeliveryLateList = append(res.Kpis.DeliveryLateList, item.DeliveryLateList[0])
		res.Kpis.DeliverySum = append(res.Kpis.DeliverySum, item.DeliverySum[0])
		res.Kpis.ActualQtyPassList = append(res.Kpis.ActualQtyPassList, item.ActualQtyPassList[0])
		res.Kpis.ActualQtyFailList = append(res.Kpis.ActualQtyFailList, item.ActualQtyFailList[0])
		res.Kpis.YellowSupplierDeliveryExpectList = append(res.Kpis.YellowSupplierDeliveryExpectList, item.YellowSupplierDeliveryExpectList[0])
		res.Kpis.RedSupplierDeliveryExpectList = append(res.Kpis.RedSupplierDeliveryExpectList, item.RedSupplierDeliveryExpectList[0])
		res.Kpis.YellowAvgSupplierDeliveryLeadtimeList = append(res.Kpis.YellowAvgSupplierDeliveryLeadtimeList, item.YellowAvgSupplierDeliveryLeadtimeList[0])
		res.Kpis.RedAvgSupplierDeliveryLeadtimeList = append(res.Kpis.RedAvgSupplierDeliveryLeadtimeList, item.RedAvgSupplierDeliveryLeadtimeList[0])
	}

	return res, nil
}

func GenerateBackWeeks(startDate time.Time, numWeeks int) []Week {
	var weeks []Week

	startOfWeek := startDate.Truncate(24 * time.Hour)
	for startOfWeek.Weekday() != time.Monday {
		startOfWeek = startOfWeek.AddDate(0, 0, -1)
	}

	for i := 0; i <= numWeeks; i++ {
		endOfWeek := startOfWeek.AddDate(0, 0, 6)

		_, weekNumber := startOfWeek.ISOWeek()

		weeks = append(weeks, Week{
			WeekNumber:  weekNumber,
			StartOfWeek: startOfWeek,
			EndOfWeek:   endOfWeek,
		})

		startOfWeek = startOfWeek.AddDate(0, 0, -7)
	}

	return weeks
}

func GetEarliestMonday(startDate time.Time, numWeeks int) time.Time {
	weeks := GenerateBackWeeks(startDate, numWeeks)
	if len(weeks) > 0 {
		return weeks[len(weeks)-1].StartOfWeek
	}
	return time.Time{}
}

func ConvertDateToDay(dateStr string) (string, error) {
	layout := "2006-01-02"

	t, err := time.Parse(layout, dateStr)
	if err != nil {
		return "", err
	}

	day := t.Weekday().String()[:3]

	return day, nil
}

func IsDateInRange(requireDate, confirmDate time.Time) (bool, error) {
	startRange := requireDate.AddDate(0, 0, -3)
	endRange := requireDate.AddDate(0, 0, 1)

	if (confirmDate.After(startRange) && confirmDate.Before(endRange)) || confirmDate.Equal(startRange) || confirmDate.Equal(endRange) {
		return true, nil
	}

	return false, nil
}
