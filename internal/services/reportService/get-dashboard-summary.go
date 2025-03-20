package reportService

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
	UserId    int      `json:"userId"`
}

type GetDashboardSummaryResponse struct {
	LabelText             string `json:"labelText"`
	TotalRequest          int    `json:"totalRequest"`
	TotalPassKpi          int    `json:"totalPassKpi"`
	TotalFailKpi          int    `json:"totalFailKpi"`
	PassKpiPercent        int    `json:"passKpiPercent"`
	TotalLate             int    `json:"totalLate"`
	TotalOntime           int    `json:"totalOntime"`
	TotalEarly            int    `json:"totalEarly"`
	TotalPassConfQtyKpi   int    `json:"totalPassConfQtyKpi"`
	TotalFailConfQtyKpi   int    `json:"totalFailConfQtyKpi"`
	TotalPassActualQtyKpi int    `json:"totalPassActualQtyKpi"`
	TotalFailActualQtyKpi int    `json:"totalFailActualQtyKpi"`
}

type DailySummaryCal struct {
	Day              string
	Date             time.Time
	RequestType      string
	MaterialID       string
	Sku              string
	SupplierCode     string
	SupplierName     string
	RequestQty       int
	ConfirmQty       int
	ConfirmQtyKPI    bool
	ConfirmDate      time.Time
	ConfirmDateKPI   bool
	TotalStock       int
	TotalConfirmQty  int
	TotalDiffQty     int
	ActualConfirmQty int
	SystemRemark     string
	ActualQtyKPI     bool
	SummaryKPI       bool
}

type JitDaily struct {
	DailyDate          time.Time
	MaterialID         string
	Sku                string
	SupplierCode       string
	SupplierName       string
	RequireQty         int
	ConfirmRequireQty  int
	ConfirmRequireDate time.Time
	UrgentQty          int
	ConfirmUrgentQty   int
	ConfirmUrgentDate  time.Time
}

type StockDay struct {
	MaterialID string
	Date       time.Time
	Qty        int
}

type Week struct {
	WeekNumber  int
	StartOfWeek time.Time
	EndOfWeek   time.Time
}

func GetDashboardSummary(c *gin.Context, jsonPayload string) (interface{}, error) {
	var req GetDashboardSummaryRequest
	var res []GetDashboardSummaryResponse

	if err := json.Unmarshal([]byte(jsonPayload), &req); err != nil {
		return nil, errors.New("failed to unmarshal JSON into struct: " + err.Error())
	}

	sqlx, err := db.ConnectSqlx(`jit_portal`)
	if err != nil {
		return nil, err
	}
	defer sqlx.Close()

	qCondSupplier := ``
	qCondMaterial := ``
	qCondWeekData := ``
	// week := 0

	// if req.WeekData != 0 {
	// 	week = req.WeekData
	// }

	// startDate := GetMondayOfCurrentWeek()
	// firstDate := GetEarliestMonday(time.Now(), week-1).AddDate(0, 0, 7)

	if len(req.Suppliers) > 0 {
		qCondSupplier = fmt.Sprintf(` and s.supplier_code in ('%s') `, strings.Join(req.Suppliers, `','`))
		// qCondSupplier = fmt.Sprintf(` and s.supplier_id in ('%s') `, strings.Join(req.Suppliers, `','`))
	}

	if len(req.Materials) > 0 {
		qCondMaterial = fmt.Sprintf(` and m.material_code in ('%s')`, strings.Join(req.Materials, `','`))
	}

	qCondWeekData = fmt.Sprintf(` and jd.daily_date >= current_date`)

	qCondUser := fmt.Sprintf(` and case when util_is_admin(%d) then true else 
        case
			when (select util_is_supplier(%d)) then
				mat.supplier_id = (select supplier_id from users where user_id = %d)
			else
				case when util_is_planner(%d) then
					(select planner_code from users where user_id = %d) like '%%' || mat.planner_code || '%%'
				else
					false
				end
			end
    	end`, req.UserId, req.UserId, req.UserId, req.UserId, req.UserId)

	queryDaily := fmt.Sprintf(`
		select jd.daily_date
			, m.material_id
			, m.material_code
			, s.supplier_code 
			, s.supplier_name
			, coalesce(jd.required_qty,0) as require_qty
			, coalesce(jd.conf_qty,0) as confirm_require_qty
			, coalesce(jd.conf_date, '1900-01-01') as confirm_require_date
			, coalesce(jd.urgent_qty,0) as urgent_qty
			, coalesce(jd.conf_urgent_qty,0) as confirm_urgent_qty
			, coalesce(jd.conf_urgent_date, '1900-01-01') as confirm_urgent_date
		from jit_daily jd
		left join materials mat on mat.material_id = jd.material_id
		left join jit_daily jd_prd on jd.original_jit_daily_id = jd_prd.jit_daily_id
		left join suppliers s on jd.supplier_id = s.supplier_id
		left join materials m on jd.material_id = m.material_id
		where 1=1 and jd.is_deleted = false 
		and (coalesce (jd.required_qty , 0) <> 0 or coalesce (jd.urgent_qty , 0) <> 0)
		%s
		%s
		%s
		%s
	`, qCondUser, qCondWeekData, qCondSupplier, qCondMaterial)
	println(queryDaily)
	rowsDaily, err := db.ExecuteQuery(sqlx, queryDaily)

	if err != nil {
		return nil, err
	}

	if len(rowsDaily) == 0 {
		return nil, nil
	}

	// println("rows from query")
	// println(len(rowsDaily))

	var jitDailys []JitDaily
	var matDay []string
	matDayMap := make(map[string]bool)
	for _, item := range rowsDaily {
		materialID := strconv.FormatInt(item["material_id"].(int64), 10)

		var daily JitDaily
		daily.MaterialID = materialID
		daily.DailyDate = item["daily_date"].(time.Time).Truncate(24 * time.Hour)
		daily.Sku = item["material_code"].(string)
		daily.SupplierCode = item["supplier_code"].(string)
		daily.SupplierName = item["supplier_name"].(string)
		daily.RequireQty = int(item["require_qty"].(float64))
		daily.ConfirmRequireQty = int(item["confirm_require_qty"].(float64))
		daily.ConfirmRequireDate = item["confirm_require_date"].(time.Time)
		daily.UrgentQty = int(item["urgent_qty"].(float64))
		daily.ConfirmUrgentQty = int(item["confirm_urgent_qty"].(float64))
		daily.ConfirmUrgentDate = item["confirm_urgent_date"].(time.Time)

		jitDailys = append(jitDailys, daily)

		if daily.ConfirmRequireQty > 0 {
			confirmDateStr := daily.ConfirmRequireDate.Format("2006-01-02")

			matDayKey := fmt.Sprintf(`%s|%s`, confirmDateStr, daily.MaterialID)
			if _, exist := matDayMap[matDayKey]; !exist {
				matDay = append(matDay, fmt.Sprintf(`('%s','%s')`, confirmDateStr, daily.MaterialID))
			}
		}

		if daily.ConfirmUrgentQty > 0 {
			confirmDateStr := daily.ConfirmUrgentDate.Format("2006-01-02")

			matDayKey := fmt.Sprintf(`%s|%s`, confirmDateStr, daily.MaterialID)
			if _, exist := matDayMap[matDayKey]; !exist {
				matDay = append(matDay, fmt.Sprintf(`('%s','%s')`, confirmDateStr, daily.MaterialID))
			}
		}
	}

	if len(matDay) == 0 {
		return nil, nil
		// return nil, errors.New(`not found materials`)
	}

	queryMatDay := fmt.Sprintf(`
		select material_id , "date" , current_qty 
		from inventory_snapshots is2
		where coalesce (current_qty,0) <> 0
		and ("date", material_id) in (%s)
	`, strings.Join(matDay, `,`))
	//println(queryMatDay)
	rowMatDay, err := db.ExecuteQuery(sqlx, queryMatDay)
	if err != nil {
		return nil, err
	}

	matDays := make(map[string]StockDay)
	for _, item := range rowMatDay {
		date := item["date"].(time.Time).Truncate(24 * time.Hour)
		dateStr := date.Format("2006-01-02")
		material := strconv.FormatInt(item["material_id"].(int64), 10)
		qty := int(item["current_qty"].(float64))

		key := fmt.Sprintf(`%s|%s`, dateStr, material)

		matDays[key] = StockDay{
			MaterialID: material,
			Date:       date,
			Qty:        qty,
		}
	}

	dailyCal, err := CalSummaryKPI(jitDailys, matDays)
	if err != nil {
		return nil, err
	}

	if len(req.Suppliers) == 1 {
		res = summaryByWeek(dailyCal, req.WeekData)
		sort.Slice(res, func(i, j int) bool {
			dateBefore, err := time.Parse("02/01/2006", res[i].LabelText)
			if err != nil {
				fmt.Println("Error parsing date:", err)
			}

			dateAfter, err := time.Parse("02/01/2006", res[j].LabelText)
			if err != nil {
				fmt.Println("Error parsing date:", err)
			}

			return dateBefore.Before(dateAfter)

		})
	} else {
		res = summaryBySupplier(dailyCal)
	}

	// println("rows from dailyCal")
	// println(len(dailyCal))

	return res, nil
}

func CalSummaryKPI(datas []JitDaily, stockDays map[string]StockDay) (map[string]DailySummaryCal, error) {
	res := make(map[string]DailySummaryCal)
	dailyConfirmMap := make(map[string][]DailySummaryCal)

	for _, dailyItem := range datas {
		day := ``
		requestDate := dailyItem.DailyDate.Truncate(24 * time.Hour)
		requestDateStr := dailyItem.DailyDate.Format("2006-01-02")
		supplierCode := dailyItem.SupplierCode
		supplierName := dailyItem.SupplierName
		materialID := dailyItem.MaterialID
		sku := dailyItem.Sku

		requireQty := dailyItem.RequireQty
		confirmRequireQty := dailyItem.ConfirmRequireQty
		confirmRequireDate := dailyItem.ConfirmRequireDate.Truncate(24 * time.Hour)
		confirmRequireDateStr := confirmRequireDate.Format("2006-01-02")

		urgentQty := dailyItem.UrgentQty
		confirmUrgentQty := dailyItem.ConfirmUrgentQty
		confirmUrgentDate := dailyItem.ConfirmUrgentDate.Truncate(24 * time.Hour)
		confirmUrgentDateStr := confirmUrgentDate.Format("2006-01-02")

		dayTemp, err := ConvertDateToDay(requestDateStr)
		if err == nil {
			day = dayTemp
		}

		if requireQty > 0 {
			confirmQtyKPI := false
			confirmDateKPI := false
			requestType := `Normal`
			tatalStock := 0
			if confirmRequireQty >= requireQty && (((confirmRequireQty-requireQty)/requireQty)*100) <= 5 {
				confirmQtyKPI = true
			}

			if ans, err := isDateInRange(requestDate, confirmRequireDate); err == nil && ans {
				confirmDateKPI = true
			}

			dailySum := DailySummaryCal{
				Day:              day,
				Date:             requestDate,
				RequestType:      requestType,
				SupplierCode:     supplierCode,
				SupplierName:     supplierName,
				MaterialID:       materialID,
				Sku:              sku,
				RequestQty:       requireQty,
				ConfirmQty:       confirmRequireQty,
				ConfirmQtyKPI:    confirmQtyKPI,
				ConfirmDate:      confirmRequireDate,
				ConfirmDateKPI:   confirmDateKPI,
				TotalStock:       tatalStock,
				TotalConfirmQty:  0,
				TotalDiffQty:     0,
				ActualConfirmQty: 0,
				SystemRemark:     "",
				ActualQtyKPI:     false,
				SummaryKPI:       false,
			}

			keyRes := fmt.Sprintf(`%s|%s|%s`, requestDateStr, requestType, materialID)
			res[keyRes] = dailySum

			if confirmRequireQty > 0 {
				keyConfirmMap := fmt.Sprintf(`%s|%s`, confirmRequireDateStr, materialID)
				dailyConfirmMap[keyConfirmMap] = append(dailyConfirmMap[keyConfirmMap], dailySum)
			}
		}

		if urgentQty > 0 {
			confirmQtyKPI := false
			confirmDateKPI := false
			requestType := `Urgent`
			tatalStock := 0

			if confirmUrgentQty >= urgentQty && (((confirmUrgentQty-urgentQty)/urgentQty)*100) <= 5 {
				confirmQtyKPI = true
			}

			if ans, err := isDateInRange(requestDate, confirmUrgentDate); err == nil && ans {
				confirmDateKPI = true
			}

			dailySum := DailySummaryCal{
				Day:              day,
				Date:             requestDate,
				RequestType:      requestType,
				SupplierCode:     supplierCode,
				SupplierName:     supplierName,
				MaterialID:       materialID,
				Sku:              sku,
				RequestQty:       urgentQty,
				ConfirmQty:       confirmUrgentQty,
				ConfirmQtyKPI:    confirmQtyKPI,
				ConfirmDate:      confirmUrgentDate,
				ConfirmDateKPI:   confirmDateKPI,
				TotalStock:       tatalStock,
				TotalConfirmQty:  0,
				TotalDiffQty:     0,
				ActualConfirmQty: 0,
				SystemRemark:     "",
				ActualQtyKPI:     false,
				SummaryKPI:       false,
			}

			keyRes := fmt.Sprintf(`%s|%s|%s`, requestDateStr, requestType, materialID)
			res[keyRes] = dailySum

			if confirmUrgentQty > 0 {
				keyConfirmMap := fmt.Sprintf(`%s|%s`, confirmUrgentDateStr, materialID)
				dailyConfirmMap[keyConfirmMap] = append(dailyConfirmMap[keyConfirmMap], dailySum)
			}
		}
	}

	for confirmKey, confirmItem := range dailyConfirmMap {
		remainMaterialStock := 0

		if _, exists := stockDays[confirmKey]; exists {
			remainMaterialStock = stockDays[confirmKey].Qty
		} else {
			continue
		}

		sort.Slice(confirmItem, func(i, j int) bool {
			if confirmItem[i].ConfirmDateKPI && confirmItem[i].ConfirmQtyKPI &&
				confirmItem[j].ConfirmDateKPI && confirmItem[j].ConfirmQtyKPI {

				if confirmItem[i].Date.Equal(confirmItem[j].Date) {
					return confirmItem[i].RequestType == "Urgent" && confirmItem[j].RequestType != "Urgent"
				}
				return confirmItem[i].Date.Before(confirmItem[j].Date)
			}

			if confirmItem[i].ConfirmDateKPI && confirmItem[i].ConfirmQtyKPI {
				return true
			}
			if confirmItem[j].ConfirmDateKPI && confirmItem[j].ConfirmQtyKPI {
				return false
			}

			if confirmItem[i].Date.Equal(confirmItem[j].Date) {
				return confirmItem[i].RequestType == "Urgent" && confirmItem[j].RequestType != "Urgent"
			}

			return confirmItem[i].Date.Before(confirmItem[j].Date)
		})

		for i, sumItem := range confirmItem {
			requestDateStr := sumItem.Date.Format("2006-01-02")
			requestType := sumItem.RequestType
			materialID := sumItem.MaterialID
			sumItemConfirmQty := sumItem.ConfirmQty
			isLastIteration := i == len(confirmItem)-1

			updateAllowcateKey := fmt.Sprintf(`%s|%s|%s`, requestDateStr, requestType, materialID)

			if updateAllowcate, exist := res[updateAllowcateKey]; exist {
				if isLastIteration {
					if remainMaterialStock == sumItemConfirmQty {
						updateAllowcate.ActualQtyKPI = true
					}

					updateAllowcate.ActualConfirmQty = remainMaterialStock

					remainMaterialStock = 0
				} else if remainMaterialStock >= sumItemConfirmQty {
					updateAllowcate.ActualQtyKPI = true
					updateAllowcate.ActualConfirmQty = sumItemConfirmQty

					remainMaterialStock -= sumItemConfirmQty
				} else if remainMaterialStock < sumItemConfirmQty {
					updateAllowcate.ActualQtyKPI = false
					updateAllowcate.ActualConfirmQty = remainMaterialStock

					remainMaterialStock = 0

					break
				}

				updateAllowcate.SystemRemark = fmt.Sprintf(`%s - seq %d`, requestDateStr, i+1)

				if updateAllowcate.ConfirmQtyKPI && updateAllowcate.ConfirmDateKPI && updateAllowcate.ActualQtyKPI {
					updateAllowcate.SummaryKPI = true
				}

				res[updateAllowcateKey] = updateAllowcate
			}
		}

		if _, exists := stockDays[confirmKey]; exists {
			updateStockDays := stockDays[confirmKey]
			updateStockDays.Qty = remainMaterialStock
			stockDays[confirmKey] = updateStockDays
		}
	}

	return res, nil
}

func summaryBySupplier(datas map[string]DailySummaryCal) []GetDashboardSummaryResponse {
	var res []GetDashboardSummaryResponse
	resMap := make(map[string]GetDashboardSummaryResponse)

	for _, item := range datas {
		key := item.SupplierCode

		totalRequest := 0
		totalPassKpi := 0
		totalFailKpi := 0
		passKpiPercent := 0
		totalLate := 0
		totalOntime := 0
		totalEarly := 0
		totalPassConfQtyKpi := 0
		totalFailConfQtyKpi := 0
		totalPassActualQtyKpi := 0
		totalFailActualQtyKpi := 0

		totalRequest++

		if item.SummaryKPI {
			totalPassKpi++
		} else {
			totalFailKpi++
		}

		if item.ConfirmQtyKPI {
			totalPassConfQtyKpi++
		} else {
			totalFailConfQtyKpi++
		}

		if item.Date.Before(item.ConfirmDate) {
			totalLate++
		}

		if item.Date == item.ConfirmDate {
			totalOntime++
		}

		if item.Date.After(item.ConfirmDate) {
			totalEarly++
		}

		if item.ActualQtyKPI {
			totalPassActualQtyKpi++
		} else {
			totalFailActualQtyKpi++
		}

		if _, exist := resMap[key]; !exist {

			if totalPassKpi != 0 && totalRequest != 0 {
				passKpiPercent = int((float64(totalPassKpi) / float64(totalRequest)) * 100)
			}

			sumItem := GetDashboardSummaryResponse{
				LabelText:             item.SupplierName,
				TotalRequest:          totalRequest,
				TotalPassKpi:          totalPassKpi,
				TotalFailKpi:          totalFailKpi,
				PassKpiPercent:        passKpiPercent,
				TotalLate:             totalLate,
				TotalOntime:           totalOntime,
				TotalEarly:            totalEarly,
				TotalPassConfQtyKpi:   totalPassConfQtyKpi,
				TotalFailConfQtyKpi:   totalFailConfQtyKpi,
				TotalPassActualQtyKpi: totalPassActualQtyKpi,
				TotalFailActualQtyKpi: totalFailActualQtyKpi,
			}

			resMap[key] = sumItem
		} else {
			sumItem := resMap[key]
			sumItem.TotalRequest += totalRequest
			sumItem.TotalPassKpi += totalPassKpi
			sumItem.TotalFailKpi += totalFailKpi
			sumItem.TotalLate += totalLate
			sumItem.TotalOntime += totalOntime
			sumItem.TotalEarly += totalEarly
			sumItem.TotalPassConfQtyKpi += totalPassConfQtyKpi
			sumItem.TotalFailConfQtyKpi += totalFailConfQtyKpi
			sumItem.TotalPassActualQtyKpi += totalPassActualQtyKpi
			sumItem.TotalFailActualQtyKpi += totalFailActualQtyKpi

			if sumItem.TotalPassKpi != 0 && sumItem.TotalRequest != 0 {
				sumItem.PassKpiPercent = int((float64(sumItem.TotalPassKpi) / float64(sumItem.TotalRequest)) * 100)
			}

			resMap[key] = sumItem
		}
	}

	for _, item := range resMap {
		res = append(res, item)
	}

	return res
}

func summaryByWeek(datas map[string]DailySummaryCal, WeekData int) []GetDashboardSummaryResponse {
	var res []GetDashboardSummaryResponse
	resMap := make(map[string]GetDashboardSummaryResponse)
	weeks := GenerateBackWeeks(GetMondayDateOfCurrentWeek().AddDate(0, 0, -1), WeekData-1)

	for _, item := range weeks {
		key := strconv.Itoa(item.WeekNumber)

		sumItem := GetDashboardSummaryResponse{
			LabelText: item.StartOfWeek.Format("02/01/2006"),
		}

		resMap[key] = sumItem
	}

	for _, item := range datas {
		_, week := item.Date.ISOWeek()
		weekStr := strconv.Itoa(week)
		key := weekStr

		totalRequest := 0
		totalPassKpi := 0
		totalFailKpi := 0
		passKpiPercent := 0
		totalLate := 0
		totalOntime := 0
		totalEarly := 0
		totalPassConfQtyKpi := 0
		totalFailConfQtyKpi := 0
		totalPassActualQtyKpi := 0
		totalFailActualQtyKpi := 0

		totalRequest++

		if item.SummaryKPI {
			totalPassKpi++
		} else {
			totalFailKpi++
		}

		if item.ConfirmQtyKPI {
			totalPassConfQtyKpi++
		} else {
			totalFailConfQtyKpi++
		}

		if item.Date.Before(item.ConfirmDate) {
			totalLate++
		}

		if item.Date == item.ConfirmDate {
			totalOntime++
		}

		if item.Date.After(item.ConfirmDate) {
			totalEarly++
		}

		if item.ActualQtyKPI {
			totalPassActualQtyKpi++
		} else {
			totalFailActualQtyKpi++
		}

		if _, exist := resMap[key]; exist {
			sumItem := resMap[key]
			sumItem.TotalRequest += totalRequest
			sumItem.TotalPassKpi += totalPassKpi
			sumItem.TotalFailKpi += totalFailKpi
			sumItem.PassKpiPercent += passKpiPercent
			sumItem.TotalLate += totalLate
			sumItem.TotalOntime += totalOntime
			sumItem.TotalEarly += totalEarly
			sumItem.TotalPassConfQtyKpi += totalPassConfQtyKpi
			sumItem.TotalFailConfQtyKpi += totalFailConfQtyKpi
			sumItem.TotalPassActualQtyKpi += totalPassActualQtyKpi
			sumItem.TotalFailActualQtyKpi += totalFailActualQtyKpi

			if sumItem.TotalPassKpi != 0 && sumItem.TotalRequest != 0 {
				sumItem.PassKpiPercent = int((float64(sumItem.TotalPassKpi) / float64(sumItem.TotalRequest)) * 100)
			}

			resMap[key] = sumItem
		}
	}

	for _, item := range resMap {
		res = append(res, item)
	}

	return res
}
