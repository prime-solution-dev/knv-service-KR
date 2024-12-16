package jitInboundService

import (
	"errors"
	"fmt"
	"jnv-jit/internal/db"
	"jnv-jit/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func UploadPlan(c *gin.Context) (interface{}, error) {
	if c.ContentType() != "multipart/form-data" {
		return nil, errors.New("incorrect content type, expected multipart/form-data")
	}

	form, err := c.MultipartForm()
	if err != nil {
		return nil, errors.New("failed to get multipart form: " + err.Error())
	}

	if len(form.File) == 0 {
		return nil, errors.New("no file found in the request")
	}

	sqlx, err := db.ConnectSqlx(`jit_portal`)
	if err != nil {
		return nil, err
	}
	defer sqlx.Close()

	plans := []RequestPlan{}
	materialCodes := []string{}
	materialCodeCheck := map[string]bool{}
	convertQty := 12.0

	for fieldName := range form.File {
		data, err := utils.ReadExcelFile(c, fieldName, ``)
		if err != nil {
			return nil, err
		}

		for _, row := range data {
			materialCode := fmt.Sprintf("%d", utils.GetDefaultValue(row, "SKU SAP", "int64").(int64))
			lineCode := utils.GetDefaultValue(row, "Line", "string").(string)
			requestQty := 0.0
			requestPlanQty := utils.GetDefaultValue(row, "Plan Qty (dz)", "float").(float64) * convertQty
			requestSubconQty := 0.0
			startPlanDateVal := utils.GetDefaultValue(row, "Start time", "string").(string)
			endPlanDateVal := utils.GetDefaultValue(row, "Finish time", "string").(string)
			var startPlanDate time.Time
			var endPlanDate *time.Time

			if startPlanDateVal != "" {
				date, err := time.Parse("2006-01-02 15:04:05", startPlanDateVal)
				if err != nil {
					return nil, fmt.Errorf("invalid convert start date: %w", err)
				}

				startPlanDate = date
			}

			if endPlanDateVal != "" {
				date, err := time.Parse("2006-01-02 15:04:05", endPlanDateVal)
				if err != nil {
					return nil, fmt.Errorf("invalid convert finish date: %w", err)
				}

				endPlanDate = &date
			}

			plan := RequestPlan{
				MaterialCode:     materialCode,
				LineCode:         lineCode,
				RequestQty:       requestQty,
				RequestPlantQty:  requestPlanQty,
				RequestSubconQty: requestSubconQty,
				PlanDate:         startPlanDate,
				EndPlanDate:      endPlanDate,
			}

			plans = append(plans, plan)

			matKey := materialCode
			if _, exist := materialCodeCheck[matKey]; !exist {
				materialCodes = append(materialCodes, materialCode)
				materialCodeCheck[matKey] = true
			}
		}
	}

	matStock, err := getMaterialStock(sqlx, materialCodes)
	if err != nil {
		return nil, err
	}

	uploadPlan := UploadPlanRequest{}
	uploadPlan.StartCal = getStartCalDate(sqlx)
	uploadPlan.MaterialStocks = matStock
	uploadPlan.RequestPlan = plans
	uploadPlan.IsBom = true
	uploadPlan.IsCheckFg = true
	uploadPlan.IsUrgentByStockDif = false

	_, err = CalculateUploadPlan(uploadPlan)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func getStartCalDate(sqlx *sqlx.DB) time.Time {
	result, err := db.ExecuteQuery(sqlx, "select start_cal_date_kr() date")

	if err != nil {
		return time.Now()
	}

	return result[0]["date"].(time.Time)
}

func getMaterialStock(sqlx *sqlx.DB, materialCodes []string) ([]MaterialStock, error) {
	matStock := []MaterialStock{}

	query := fmt.Sprintf(`
		select m.material_code
			, coalesce (m.current_qty , 0) qty
		from materials m 
		where m.current_qty <> 0
		and m.material_code in ('%s')
	`, strings.Join(materialCodes, `','`))
	rows, err := db.ExecuteQuery(sqlx, query)
	if err != nil {
		return nil, err
	}

	for _, item := range rows {
		materialCode := item["material_code"].(string)
		qty := item["qty"].(float64)

		newMatStock := MaterialStock{
			MaterialCode:  materialCode,
			StockPlantQty: qty,
		}

		matStock = append(matStock, newMatStock)
	}

	return matStock, nil
}
