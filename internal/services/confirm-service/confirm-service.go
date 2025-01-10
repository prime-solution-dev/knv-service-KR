package confirmservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"jnv-jit/internal/db"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"gorm.io/gorm"
)

type ConfirmDataList map[string][]ConfirmData

func Confirm(c *gin.Context, jsonPayload string) (interface{}, error) {
	var reqBody ConfirmRequestBody

	if err := json.Unmarshal([]byte(jsonPayload), &reqBody); err != nil {
		return nil, errors.New("failed to unmarshal JSON into struct: " + err.Error())
	}

	req := reqBody.Data

	sqlx, err := db.ConnectSqlx(`jit_portal`)
	if err != nil {
		return nil, err
	}
	defer sqlx.Close()

	gorm, err := db.ConnectGORM(`jit_portal`)
	if err != nil {
		return nil, err
	}
	defer db.CloseGORM(gorm)

	startDate := getStartDate(sqlx)

	confirmData, err := getMetaData(sqlx, req)
	if err != nil {
		return nil, err
	}

	confirmData, err = allocateConfirm(req, confirmData)
	if err != nil {
		return nil, err
	}

	tx := gorm.Begin()

	ConfirmMinMatDate, err := updateConfirm(tx, confirmData)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
	}

	tx = gorm.Begin()

	materialUpdates := []string{}
	for _, mat := range ConfirmMinMatDate {
		materialUpdates = append(materialUpdates, fmt.Sprintf("'%s'", mat.Materials))
	}

	confirmDetailData, err := getConfirmData(sqlx, startDate, materialUpdates)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = recalActual(tx, sqlx, ConfirmMinMatDate, startDate, confirmDetailData)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
	}

	filename := reqBody.Filename
	uploadRow := 0
	uploadStatus := err == nil

	uploadReason := "Success"
	if err != nil {
		uploadReason = err.Error()
	}

	// userIDValue := reqBody.UserId

	sql := fmt.Sprintf(`INSERT INTO upload_logs (
		master_name,
		type,
		file_name,
		upload_row,
		status,
		percent,
		import_date,
		last_update_date,
		upload_reason,
		action_by
	) VALUES (
		'%s', '%s', '%s', %d, %t, %d, '%s', '%s', '%s', %d
	)`,
		"jit-daily-confirm-delivery",
		"-",
		filename,
		uploadRow,
		uploadStatus,
		100,
		time.Now().Format(time.RFC3339),
		time.Now().Format(time.RFC3339),
		uploadReason, reqBody.UserId)

	println(sql)
	_, err = db.ExecuteQuery(sqlx, sql)

	if err != nil {
		return nil, err
	}

	return ConfirmResponse{
		Success: uploadStatus,
		Message: uploadReason,
	}, nil
}

func getStartDate(sqlx *sqlx.DB) time.Time {
	result, err := db.ExecuteQuery(sqlx, "select start_cal_date_kr() date") //todo: change function name

	if err != nil {
		return time.Now()
	}

	return result[0]["date"].(time.Time)
}

func getConfirmData(sqlx *sqlx.DB, startDate time.Time, materialUpdates []string) (map[string]ConfirmDetailData, error) {
	result := make(map[string]ConfirmDetailData)
	startDateStr := startDate.Format("2006-01-02")

	sql := fmt.Sprintf(`select
                        max(main.original_jit_daily_id) original_jit_daily_id,
                        max(main.material_id) material_id,
                        max(mat.material_code) material_code,
                        max(main.daily_date) daily_date,
                        max(main.begin_stock) begin_stock,
                        sum(main.product_qty) product_qty,
                        sum(main.conf_qty) conf_qty,
                        sum(main.conf_urgent_qty) conf_urgent_qty,
                        coalesce(max(main.conf_date), '1990-01-01') conf_date,
                        coalesce(max(main.conf_urgent_date), '1990-01-01') conf_urgent_date
                    from jit_daily main
					inner join materials mat on mat.material_id = main.material_id
                    where
                        main.is_deleted = false and
                        (main.conf_date >= '%s' or main.conf_urgent_date >= '%s') and
						mat.material_code in (%s)
                    group by main.material_id, main.daily_date
                    order by main.daily_date`, startDateStr, startDateStr, strings.Join(materialUpdates, ","))

	data, err := db.ExecuteQuery(sqlx, sql)
	println(sql)
	if err != nil {
		return nil, fmt.Errorf("can not fetch confirm data")
	}

	for _, item := range data {
		materialCode := item["material_code"].(string)
		confDate := (item["conf_date"].(time.Time))
		confDateStr := confDate.Format("2006-01-02")
		urgentConfDate := (item["conf_date"].(time.Time))
		urgentConfDateStr := urgentConfDate.Format("2006-01-02")
		confirmQty := item["conf_qty"].(float64)
		confirmUrgentQty := item["conf_urgent_qty"].(float64)

		key := fmt.Sprintf("%s|%s", materialCode, confDateStr)
		confDataMapValue, exists := result[key]
		if !exists {
			confDataMapValue = ConfirmDetailData{
				MaterialID:    item["material_id"].(int64),
				DailyDate:     item["daily_date"].(time.Time),
				BeginStock:    item["begin_stock"].(float64),
				ProductQty:    item["product_qty"].(float64),
				ConfQty:       0,
				ConfUrgentQty: 0,
			}
		}

		if confirmQty > 0 {
			confDataMapValue.ConfQty += confirmQty
			confDataMapValue.ConfDate = &confDate
		}

		result[key] = confDataMapValue

		key = fmt.Sprintf("%s|%s", materialCode, urgentConfDateStr)
		urgentConfDataMapValue, exists := result[key]
		if !exists {
			confDataMapValue = ConfirmDetailData{
				OriginalJitDailyID: item["original_jit_daily_id"].(int64),
				MaterialID:         item["material_id"].(int64),
				DailyDate:          item["daily_date"].(time.Time),
				BeginStock:         item["begin_stock"].(float64),
				ProductQty:         item["product_qty"].(float64),
				ConfQty:            0,
				ConfUrgentQty:      0,
			}
		}

		if confirmUrgentQty > 0 {
			confDataMapValue.ConfQty += confirmQty
			confDataMapValue.ConfUrgentDate = &urgentConfDate
		}

		result[key] = urgentConfDataMapValue
	}

	return result, nil
}

func getMetaData(sqlx *sqlx.DB, req []ConfirmRequest) (ConfirmDataList, error) {
	var matDate []string
	matDateCheck := make(map[string]bool)
	matDateData := make(ConfirmDataList)

	for _, item := range req {
		materialCode := *item.MaterialCode
		date := time.Time(*item.RequiredDate).Format("2006-01-02")

		key := fmt.Sprintf("%s|%s", materialCode, date)

		if _, exists := matDateCheck[key]; !exists {
			matDate = append(matDate, fmt.Sprintf("('%s', '%s')", materialCode, date))
			matDateCheck[key] = true
		}
	}

	sql := fmt.Sprintf(`
        select
            coalesce(main.material_id, 0) material_id,
            mat.material_code,
            coalesce(main.daily_date, '1990-01-01') required_date,
            coalesce(original_main.daily_time, '1990-01-01 00:00') daily_time,
            coalesce(main.required_qty, 0) required_qty,
            coalesce(main.urgent_qty, 0) urgent_qty,
            coalesce(main.jit_daily_id, 0) jit_daily_id,
            coalesce(main.line_id, 0) line_id
        from jit_daily main
        left join materials mat on mat.material_id = main.material_id
		left join jit_daily original_main on original_main.jit_daily_id = main.original_jit_daily_id
        where
            main.is_deleted = false and
            ((main.required_qty > 0 or main.urgent_qty > 0) or (main.conf_qty > 0 and main.required_qty = 0) or (main.conf_urgent_qty > 0 and main.urgent_qty = 0)) and
            (mat.material_code, main.daily_date) in (%s)
    `, strings.Join(matDate, ","))

	println(sql)

	data, err := db.ExecuteQuery(sqlx, sql)

	if err != nil {
		return nil, fmt.Errorf("can not fetch meta data")
	}

	for _, mat := range data {
		materialCode := mat["material_code"].(string)
		date := (mat["required_date"].(time.Time)).Format("2006-01-02")

		key := fmt.Sprintf("%s|%s", materialCode, date)
		if _, exists := matDateData[key]; !exists {
			matDateData[key] = []ConfirmData{}
		}

		matDateData[key] = append(matDateData[key], ConfirmData{
			MaterialId:   mat["material_id"].(int64),
			MaterialCode: mat["material_code"].(string),
			RequiredDate: mat["required_date"].(time.Time),
			RequireTime:  mat["daily_time"].(time.Time),
			DailyTime:    mat["daily_time"].(time.Time),
			RequiredQty:  mat["required_qty"].(float64),
			UrgentQty:    mat["urgent_qty"].(float64),
			JitDailyID:   mat["jit_daily_id"].(int64),
			LineID:       mat["line_id"].(int64),
		})
	}

	return matDateData, nil
}

func allocateConfirm(req []ConfirmRequest, confirmDataMap ConfirmDataList) (ConfirmDataList, error) {
	newConfirmDataMap := confirmDataMap

	for _, reqData := range req {
		materialCode := *reqData.MaterialCode
		date := time.Time(*reqData.RequiredDate).Format("2006-01-02")

		key := fmt.Sprintf("%s|%s", materialCode, date)
		confirmData, exists := confirmDataMap[key]

		if !exists {
			return nil, fmt.Errorf("not found confirm data for material code %s and date %s", materialCode, date)
		}

		sort.Slice(confirmData, func(a, b int) bool {
			if confirmData[a].RequiredDate.Equal(confirmData[b].RequireTime) {
				if confirmData[a].LineID == confirmData[b].LineID {
					return confirmData[a].RequiredQty < confirmData[b].RequiredQty
				}

				return confirmData[a].LineID < confirmData[b].LineID
			}

			return confirmData[a].RequiredDate.Before(confirmData[b].RequireTime)
		})

		remainQty := reqData.ConfQty

		for index, _ := range confirmData {
			if *reqData.DailyType == "" {
				return nil, fmt.Errorf("daily type is required")
			}

			var confirmValue *float64 = (*float64)(reqData.ConfQty)
			isUrgentType := *reqData.DailyType == "Urgent"

			if remainQty != nil && *remainQty < CustomFloat64(*confirmValue) || index+1 == len(confirmData) {
				confirmValue = (*float64)(remainQty)
			}

			confirmDate := (*time.Time)(reqData.ConfDate)

			if isUrgentType {
				newConfirmDataMap[key][index].ConfirmUrgentQty = *confirmValue
				newConfirmDataMap[key][index].UrgentDate = confirmDate
			} else {
				newConfirmDataMap[key][index].ConfirmQty = *confirmValue
				newConfirmDataMap[key][index].ConfirmDate = confirmDate
			}

			*remainQty = *remainQty - CustomFloat64(*confirmValue)

			if *remainQty == 0 {
				break
			}
		}

	}

	return newConfirmDataMap, nil
}

func updateConfirm(gorm *gorm.DB, confirmDataMap ConfirmDataList) ([]ConfirmMinMatDate, error) {
	result := []ConfirmMinMatDate{}
	resultAddList := make(map[string]bool)
	clearMatList := make(map[string]bool)

	for _, confirmData := range confirmDataMap {
		for _, confirmItem := range confirmData {
			key := fmt.Sprintf("%d|%s", confirmItem.MaterialId, confirmItem.RequiredDate.Format("2006-01-02"))
			if exists := clearMatList[key]; !exists {
				clearPayload := map[string]any{
					"conf_qty":         0,
					"conf_date":        nil,
					"conf_urgent_qty":  0,
					"conf_urgent_date": nil,
				}

				err := gorm.Model(&JitDaily{}).Where("material_id = ? and daily_date = ?", confirmItem.MaterialId, confirmItem.RequiredDate.Format("2006-01-02")).Updates(clearPayload).Error
				if err != nil {
					return nil, err
				}

				clearMatList[key] = true
			}

			updateData := map[string]any{
				"updated_by":       0,
				"updated_date":     time.Now(),
				"conf_qty":         0,
				"conf_date":        nil,
				"conf_urgent_qty":  0,
				"conf_urgent_date": nil,
			}

			if confirmItem.ConfirmQty != 0 && confirmItem.ConfirmDate != nil {
				updateData["conf_qty"] = confirmItem.ConfirmQty
				updateData["conf_date"] = confirmItem.ConfirmDate
			}

			if confirmItem.ConfirmUrgentQty != 0 && confirmItem.ConfirmDate != nil {
				updateData["conf_urgent_qty"] = confirmItem.ConfirmQty
				updateData["conf_urgent_date"] = confirmItem.ConfirmDate
			}

			err := gorm.Model(&JitDaily{}).Where("jit_daily_id = ?", confirmItem.JitDailyID).Updates(updateData).Error

			if err != nil {
				return nil, err
			}

			if _, exists := resultAddList[confirmItem.MaterialCode]; !exists && (confirmItem.ConfirmDate != nil || confirmItem.UrgentDate != nil) {
				if confirmItem.ConfirmDate != nil {
					result = append(result, ConfirmMinMatDate{
						MinDate:   confirmItem.ConfirmDate.AddDate(0, 0, -1),
						Materials: confirmItem.MaterialCode,
					})
				}

				if confirmItem.UrgentDate != nil {
					result = append(result, ConfirmMinMatDate{
						MinDate:   confirmItem.UrgentDate.AddDate(0, 0, -1),
						Materials: confirmItem.MaterialCode,
					})
				}
				resultAddList[confirmItem.MaterialCode] = true
			}
		}
	}

	return result, nil
}

func recalActual(tx *gorm.DB, sqlx *sqlx.DB, data []ConfirmMinMatDate, startDate time.Time, confirmData map[string]ConfirmDetailData) error {
	var jitDailyConfirmDetail []JitBaseConfirmDetail
	// jitDailyConfirmDetailMap := make(map[string]JitBaseConfirmDetail)
	endOfStockMap := make(map[string]float64)

	qSql := ""

	for index, minDateMat := range data {
		if index != 0 {
			qSql += " or "
		}

		qSql += fmt.Sprintf("(mat.material_code = '%s' and main.daily_date >= '%s')", minDateMat.Materials, startDate.Format("2006-01-02"))
	}

	sql := fmt.Sprintf(`select
                        coalesce(max(main.material_id), 0) material_id,
                        max(main.daily_date) daily_date,
                        coalesce(max(main.begin_stock), 0) begin_stock,
                        coalesce(sum(main.product_qty), 0) product_qty,
                        coalesce(sum(main.conf_qty), 0) conf_qty,
                        coalesce(sum(main.conf_urgent_qty), 0) conf_urgent_qty,
                        max(main.conf_date) conf_date,
                        max(main.conf_urgent_date) conf_urgent_date,
						coalesce(max(mat.material_code), '') material_code,
						coalesce(max(main.end_of_stock), 0) end_of_stock
                    from jit_daily main
					inner join materials mat on mat.material_id = main.material_id
                    where
                        main.is_deleted = false and
						(%s)
                    group by main.material_id, main.daily_date
                    order by main.material_id, main.daily_date`, qSql)

	items, err := db.ExecuteQuery(sqlx, sql)
	println(sql)
	if err != nil {
		return fmt.Errorf("can not fetch confirm data")
	}

	for _, confirmDetail := range items {
		materialCode := confirmDetail["material_code"].(string)
		date := confirmDetail["daily_date"].(time.Time).Format("2006-01-02")

		var confDate *time.Time
		if val, ok := confirmDetail["conf_date"].(time.Time); ok {
			confDate = &val
		} else if val, ok := confirmDetail["conf_date"].(*time.Time); ok {
			confDate = val
		}

		var confUrgentDate *time.Time
		if val, ok := confirmDetail["conf_urgent_date"].(time.Time); ok {
			confUrgentDate = &val
		} else if val, ok := confirmDetail["conf_urgent_date"].(*time.Time); ok {
			confUrgentDate = val
		}

		detailData := JitBaseConfirmDetail{
			MaterialCode:   materialCode,
			MaterialID:     confirmDetail["material_id"].(int64),
			DailyDate:      confirmDetail["daily_date"].(time.Time),
			BeginStock:     confirmDetail["begin_stock"].(float64),
			ProductQty:     confirmDetail["product_qty"].(float64),
			ConfQty:        confirmDetail["conf_qty"].(float64),
			ConfUrgentQty:  confirmDetail["conf_urgent_qty"].(float64),
			ConfDate:       confDate,
			ConfUrgentDate: confUrgentDate,
			EndOfStock:     confirmDetail["end_of_stock"].(float64),
		}

		key := fmt.Sprintf("%s|%s", materialCode, date)

		// jitDailyConfirmDetailMap[key] = detailData
		jitDailyConfirmDetail = append(jitDailyConfirmDetail, detailData)
		endOfStockMap[key] = confirmDetail["end_of_stock"].(float64)
	}

	sort.Slice(jitDailyConfirmDetail, func(i, j int) bool {
		if jitDailyConfirmDetail[i].MaterialID == jitDailyConfirmDetail[j].MaterialID {
			return jitDailyConfirmDetail[i].DailyDate.Before(jitDailyConfirmDetail[j].DailyDate)
		}
		return jitDailyConfirmDetail[i].MaterialID < jitDailyConfirmDetail[j].MaterialID
	})

	for _, jitDaily := range jitDailyConfirmDetail {
		if jitDaily.DailyDate.Before(startDate) {
			continue
		}

		dateStr := jitDaily.DailyDate.Format("2006-01-02")
		mainKey := fmt.Sprintf("%s|%s", jitDaily.MaterialCode, dateStr)
		var endOfStock float64 = 0
		var confQty float64 = 0
		var urgentQty float64 = 0

		key := fmt.Sprintf("%s|%s", jitDaily.MaterialCode, jitDaily.DailyDate.Format("2006-01-02"))
		if confQtyValue, exists := confirmData[key]; exists {
			confQty = confQtyValue.ConfQty
			urgentQty = confQtyValue.ConfUrgentQty
		}

		if jitDaily.DailyDate.Equal(startDate) {
			endOfStock = jitDaily.BeginStock - jitDaily.ProductQty + confQty + urgentQty
		} else {
			previousDateStr := jitDaily.DailyDate.AddDate(0, 0, -1).Format("2006-01-02")

			key := fmt.Sprintf("%s|%s", jitDaily.MaterialCode, previousDateStr)

			endOfStockMapValue, exists := endOfStockMap[key]
			var beforeLast float64 = 0

			if exists {
				beforeLast = endOfStockMapValue
			}

			endOfStock = beforeLast - jitDaily.ProductQty + confQty + urgentQty
		}

		// jitDailyConfirmDetail[index].EndOfStock = endOfStock
		endOfStockMap[mainKey] = endOfStock

		err := tx.Model(&JitDaily{}).Where("material_id = ? and daily_date = ?", jitDaily.MaterialID, jitDaily.DailyDate.Format("2006-01-02")).Updates(map[string]interface{}{
			"end_of_stock": endOfStock,
		}).Error

		if err != nil {
			return err
		}
	}

	return nil
}
