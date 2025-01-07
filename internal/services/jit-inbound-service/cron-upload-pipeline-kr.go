package jitInboundService

import (
	"encoding/json"
	"errors"
	"fmt"
	"jnv-jit/internal/cronjob"
	"jnv-jit/internal/db"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UploadPlanPipelineKrRequest struct {
	StartFileDate   time.Time `json:"start_file_date"`
	StartCalDate    time.Time `json:"start_cal_date"`
	StockPath       string    `json:"stock_path"`
	StockPrefixFile string    `json:"stock_prefix_file"`
	PlanPath        string    `json:"plan_path"`
	PlanPrefixFile  string    `json:"plan_prefix_file"`
}

func init() {
	cronjob.RegisterJob("upload-pipeline-kr-sun", UploadPlanPipelineKrCron, `0 20 * * 0`)
	cronjob.RegisterJob("upload-pipeline-kr-tue", UploadPlanPipelineKrCron, `0 6 * * 2`)
	cronjob.RegisterJob("upload-pipeline-kr-thu", UploadPlanPipelineKrCron, `0 6 * * 4`)

}

func UploadPlanPipelineKrCron() {
	sqlx, _ := db.ConnectSqlx(`jit_portal`)
	defer sqlx.Close()

	startFileDate := time.Now().Truncate(24 * time.Hour)
	startCalDate := GetStartCalDate(sqlx).Truncate(24 * time.Hour)
	stockPath := `/Users/m4ru/Documents/Work/Prime/FileTest/JIT`
	stockPrefixFile := `LX02_`
	planPath := `/Users/m4ru/Documents/Work/Prime/FileTest/JIT`
	planPrefixFile := `ZM35_`

	ProcessUploadPipelineKr(startFileDate, startCalDate, stockPath, stockPrefixFile, planPath, planPrefixFile)
}

func UploadPlanPipelineKr(c *gin.Context, jsonPayload string) (interface{}, error) {
	var req UploadPlanPipelineKrRequest

	if err := json.Unmarshal([]byte(jsonPayload), &req); err != nil {
		return nil, errors.New("failed to unmarshal JSON into struct: " + err.Error())
	}

	startFileDate := req.StartFileDate
	startCalDate := req.StartCalDate
	stockPath := req.StockPath
	stockPrefixFile := req.StockPrefixFile
	planPath := req.PlanPath
	planPrefixFile := req.PlanPrefixFile

	err := ProcessUploadPipelineKr(startFileDate, startCalDate, stockPath, stockPrefixFile, planPath, planPrefixFile)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func ProcessUploadPipelineKr(startFileDate, startCalDate time.Time, stockPath string, stockPrefixFile string, planPath string, planPrefixFile string) error {
	stockFilePath, err := FindLatestFileWithPrefix(stockPath, stockPrefixFile)
	if err != nil {
		return err
	}

	stockDatas, err := ReadCsvFile(stockFilePath)
	if err != nil {
		return err
	}

	matStockMap, err := ReadStock(stockDatas)
	if err != nil {
		return err
	}

	planFilePath, err := FindLatestFileWithPrefix(planPath, planPrefixFile)
	if err != nil {
		return err
	}

	planDatas, err := ReadPlainText(planFilePath)
	if err != nil {
		return err
	}

	planMap, matStockMap, err := ReadPlan(planDatas, matStockMap, startFileDate)
	if err != nil {
		return err
	}

	matStock := []MaterialStock{}
	for _, item := range matStockMap {
		matStock = append(matStock, item)
	}

	plans := []RequestPlan{}
	for _, item := range planMap {
		plans = append(plans, item)
	}

	gormx, _ := db.ConnectGORM(`jit_portal`)
	defer db.CloseGORM(gormx)

	updateFunc := func(gorm *gorm.DB, matUpdateItems []MaterialStock) {
		tx := gorm.Begin()

		for _, matUpdate := range matUpdateItems {
			tx.Table("materials").Where("material_code = ?", matUpdate.MaterialCode).Updates(map[string]any{
				"current_qty":  matUpdate.StockPlantQty + matUpdate.StockSubconQty,
				"updated_date": time.Now().Format(time.DateTime),
			})
		}

		err := tx.Commit().Error
		if err != nil {
			tx.Rollback()
		}

	}

	var matUpdateList []MaterialStock

	for index, matItem := range matStock {

		matUpdateList = append(matUpdateList, matItem)

		if len(matUpdateList) >= 500 || index == len(matStock)-1 {
			updateFunc(gormx, matUpdateList)
			matUpdateList = []MaterialStock{}
		}
	}

	uploadPlan := UploadPlanRequest{}
	uploadPlan.StartCal = startCalDate //todo get start cal
	uploadPlan.MaterialStocks = matStock
	uploadPlan.RequestPlan = plans
	uploadPlan.IsBom = false
	uploadPlan.IsCheckFg = false
	uploadPlan.IsUrgentByStockDif = false

	_, err = CalculateUploadPlan(uploadPlan)
	if err != nil {
		return err
	}

	return nil
}

func ReadStock(datas []map[string]interface{}) (map[string]MaterialStock, error) {
	matMap := map[string]MaterialStock{}
	ignoreSs := []string{`S`}
	ignoreTypes := []string{`901`, `902`, `911`, `914`, `916`, `921`, `922`, `998`, `999`, `REW`}

	for _, data := range datas {
		materialCode := data["Material"].(string)
		s := data["S"].(string)
		typ := data["Typ"].(string)
		stockQtyStr := data["Avail.stock"].(string)

		if strings.HasSuffix(stockQtyStr, "-") {
			stockQtyStr = strings.Replace(stockQtyStr, "-", "", -1)
			stockQtyStr = "-" + stockQtyStr
		}

		materialCodeFloat, err := strconv.ParseFloat(materialCode, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing float: %w", err)
		}

		materialCode = strconv.FormatFloat(materialCodeFloat, 'f', -1, 64)

		stockQty, err := strconv.ParseFloat(stockQtyStr, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing float: %w", err)
		}

		if !contains(ignoreSs, s) && !contains(ignoreTypes, typ) {
			key := materialCode
			mat, exist := matMap[key]

			if !exist {
				mat = MaterialStock{
					MaterialCode:  materialCode,
					StockPlantQty: 0,
				}
			}

			mat.StockPlantQty += stockQty

			matMap[key] = mat
		}
	}

	return matMap, nil
}

func ReadPlan(datas []map[string]interface{}, matStockMap map[string]MaterialStock, currentDate time.Time) (map[string]RequestPlan, map[string]MaterialStock, error) {
	planMap := map[string]RequestPlan{}

	for _, data := range datas {
		materialCode := data["Col2"].(string)
		lineCode := ""
		subconStockQtyStr := data["Col9"].(string)
		condition := data["Col11"].(string)
		numColumns := len(data) //todo ทำไมมันขาดไปอัน
		startCol := 11
		endCol := numColumns

		subconStockQty, err := strconv.ParseFloat(subconStockQtyStr, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("error parsing float: %w", err)
		}

		if !(condition == `Issues-Plant` || condition == `Issues-SubCon`) {
			continue
		}

		matStockKey := materialCode
		matStock, matStockExist := matStockMap[matStockKey]
		if !matStockExist {
			matStock = MaterialStock{
				MaterialCode:   materialCode,
				StockPlantQty:  0,
				StockSubconQty: 0,
			}
		}

		matStock.StockSubconQty = subconStockQty
		matStockMap[matStockKey] = matStock

		for i := startCol; i < endCol; i++ {
			planDate := currentDate.AddDate(0, 0, i-startCol)
			planDateTrunc := planDate.Truncate(24 * time.Hour)
			planDateStr := planDateTrunc.Format("2006-01-02")
			qtyStr := data[fmt.Sprintf(`Col%d`, (i+1))].(string)
			plantQty := 0.0
			subconQty := 0.0

			qty, err := strconv.ParseFloat(qtyStr, 64)
			if err != nil {
				return nil, nil, fmt.Errorf("error parsing float: %w", err)
			}

			if qty == 0 {
				continue
			}

			key := fmt.Sprintf(`%s|%s|%s`, planDateStr, materialCode, lineCode)

			if condition == `Issues-Plant` {
				plantQty = qty
			} else if condition == `Issues-SubCon` {
				subconQty = qty
			}

			plan, planExist := planMap[key]
			if !planExist {
				plan = RequestPlan{
					MaterialCode:     materialCode,
					LineCode:         lineCode,
					PlanDate:         planDate,
					RequestPlantQty:  0,
					RequestSubconQty: 0,
				}
			}

			plan.RequestPlantQty += plantQty
			plan.RequestSubconQty += subconQty
			planMap[key] = plan
		}
	}

	return planMap, matStockMap, nil
}
