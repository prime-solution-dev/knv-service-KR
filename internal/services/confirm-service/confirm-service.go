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
	var req []ConfirmRequest

	if err := json.Unmarshal([]byte(jsonPayload), &req); err != nil {
		return nil, errors.New("failed to unmarshal JSON into struct: " + err.Error())
	}

	sqlx, err := db.ConnectSqlx(`jit_portal`)
	if err != nil {
		return nil, err
	}
	defer sqlx.Close()

	gorm, err := db.ConnectGORM(`jit_portal`)
	if err != nil {
		return nil, err
	}

	confirmData, err := GetMetaData(sqlx, req)
	if err != nil {
		return nil, err
	}

	confirmData, err = AllocateConfirm(req, confirmData)
	if err != nil {
		return nil, err
	}

	ConfirmMinMatDate, err := UpdateConfirm(gorm, confirmData)
	if err != nil {
		return nil, err
	}

	startDate := getStartDate(sqlx)
	err = RecalActual(gorm, ConfirmMinMatDate, startDate)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func GetMetaData(sqlx *sqlx.DB, req []ConfirmRequest) (ConfirmDataList, error) {
	var matDate []string
	matDateCheck := make(map[string]bool)
	matDateData := make(ConfirmDataList)

	for _, item := range req {
		materialCode := item.MaterialCode
		date := item.RequiredDate.Format("2006-01-02")

		key := fmt.Sprintf("%s|%s", materialCode, date)

		if _, exists := matDateCheck[key]; !exists {
			matDate = append(matDate, fmt.Sprintf("('%s', '%s')", materialCode, date))
			matDateCheck[key] = true
		}
	}

	sql := fmt.Sprintf(`
        select
            main.material_id,
            mat.material_code,
            main.daily_date required_date,
            original_main.daily_time,
            main.required_qty required_qty,
            main.urgent_qty,
            main.jit_daily_id,
            main.line_id
        from jit_daily main
        left join materials mat on mat.material_id = main.material_id
        where
            main.is_deleted = false and
            main.daily_date = loop_data.required_date and
            (main.required_qty > 0 or main.urgent_qty > 0) and
            (mat.material_code, main.daily_date) in (%s)
    `, strings.Join(matDate, ","))

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
			MaterialId:   mat["material_id"].(int),
			RequiredDate: mat["required_date"].(time.Time),
			RequireTime:  mat["daily_time"].(time.Time),
			DailyTime:    mat["daily_time"].(string),
			RequiredQty:  mat["required_qty"].(float64),
			UrgentQty:    mat["urgent_qty"].(float64),
			JitDailyID:   mat["jit_daily_id"].(int),
			LineID:       mat["line_id"].(int),
		})
	}

	return matDateData, nil
}

func AllocateConfirm(req []ConfirmRequest, confirmDataMap ConfirmDataList) (ConfirmDataList, error) {
	for _, reqData := range req {
		materialCode := reqData.MaterialCode
		date := reqData.RequiredDate.Format("2006-01-02")

		key := fmt.Sprintf(materialCode, date)
		confirmData, exists := confirmDataMap[key]

		if !exists {
			return nil, fmt.Errorf("not found mapping")
		}

		sort.Slice(confirmData, func(a, b int) bool {
			if confirmData[a].RequiredDate.Equal(confirmData[b].RequireTime) {
				return confirmData[a].LineID < confirmData[b].LineID
			}

			return confirmData[a].RequiredDate.Before(confirmData[b].RequireTime)
		})

		remainQty := reqData.ConfQty

		if remainQty == 0 {
			continue
		}

		for index, confirmItem := range confirmData {
			confirmValue := confirmItem.RequiredQty
			isUrgentType := reqData.DailyType == "Urgent"

			if isUrgentType {
				confirmValue = confirmItem.ConfirmUrgentQty
			}

			if remainQty < confirmValue || index+1 == len(confirmData) {
				confirmValue = remainQty
			}

			if isUrgentType {
				confirmDataMap[key][index].ConfirmUrgentQty = confirmValue
				confirmDataMap[key][index].UrgentDate = reqData.RequiredDate
			} else {
				confirmDataMap[key][index].ConfirmQty = confirmValue
				confirmDataMap[key][index].ConfirmDate = reqData.RequiredDate
			}

			remainQty = remainQty - confirmValue

			if remainQty == 0 {
				break
			}
		}

	}

	return confirmDataMap, nil
}

func UpdateConfirm(gorm *gorm.DB, confirmDataMap ConfirmDataList) ([]ConfirmMinMatDate, error) {
	result := []ConfirmMinMatDate{}
	resultAddList := make(map[string]bool)

	tx := gorm.Begin()

	for _, confirmData := range confirmDataMap {
		for _, confirmItem := range confirmData {
			err := gorm.Model(&JitDaily{}).Where("jit_daily_id = ?", confirmItem.JitDailyID).Updates(map[string]interface{}{
				"conf_qty":         confirmItem.ConfirmQty,
				"conf_urgent_qty":  confirmItem.ConfirmUrgentQty,
				"conf_date":        confirmItem.ConfirmDate,
				"conf_urgent_date": confirmItem.UrgentDate,
				"updated_by":       0,
				"updated_date":     time.Now(),
			}).Error

			if err != nil {
				tx.Rollback()
				return nil, err
			}

			if _, exists := resultAddList[confirmItem.MaterialCode]; !exists {
				result = append(result, ConfirmMinMatDate{
					MinDate:   confirmItem.ConfirmDate.AddDate(0, 0, -1),
					Materials: confirmItem.MaterialCode,
				})
				resultAddList[confirmItem.MaterialCode] = true
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return result, nil
}

func getStartDate(sqlx *sqlx.DB) time.Time {
	result, err := db.ExecuteQuery(sqlx, "select start_cal_date_kr() date")

	if err != nil {
		return time.Now()
	}

	return result[0]["date"].(time.Time)
}

func RecalActual(gorm *gorm.DB, data []ConfirmMinMatDate, startData time.Time) error {
	tx := gorm.Begin()

	for _, item := range data {
		var jitDailies []JitDaily

		tx.Model(&JitDaily{}).Where("material_code = ? and daily_date >= ?", item.Materials, item.MinDate).Find(&jitDailies)

		err := tx.Model(&JitDaily{}).Find(&jitDailies).Error
		if err != nil {
			tx.Rollback()
			return err
		}

		for index, jitDaily := range jitDailies {
			if index == 0 && len(jitDailies) > 1 {
				continue
			}

			var endOfStock float64 = 0

			if jitDaily.DailyDate.Equal(startData) {
				endOfStock = jitDaily.BeginStock - jitDaily.ProductQty + jitDaily.ConfQty + jitDaily.UrgentQty
			} else {
				endOfStock = jitDailies[index-1].BeginStock - jitDaily.ProductQty + jitDaily.ConfQty + jitDaily.UrgentQty
			}

			err := tx.Model(&JitDaily{}).Where("jit_daily_id = ?", jitDaily.JitDailyID).Updates(map[string]interface{}{
				"end_of_stock": endOfStock,
			}).Error

			if err != nil {
				tx.Rollback()
			}
		}
	}

	tx.Commit()

	return nil
}
