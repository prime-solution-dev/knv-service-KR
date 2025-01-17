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
			materialCode := fmt.Sprintf("%.0f", utils.GetDefaultValue(row, "SKU SAP", "float64").(float64))

			if materialCode == "0" {
				continue
			}

			lineCode := utils.GetDefaultValue(row, "Line", "string").(string)
			requestQty := 0.0
			requestPlanQty := utils.GetDefaultValue(row, "Plan Qty (dz)", "float64").(float64) * convertQty
			requestSubconQty := 0.0

			var startPlanDate time.Time
			var endPlanDate *time.Time

			startPlanDate = utils.GetDefaultValue(row, "Start time", "datetime").(time.Time)
			endPlanDate = &startPlanDate

			if endTimeVal, ok := utils.GetDefaultValue(row, "Finish time", "datetime").(time.Time); ok && !endTimeVal.IsZero() {
				endPlanDate = &endTimeVal
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
	uploadPlan.StartCal = GetStartCalDate(sqlx)
	uploadPlan.MaterialStocks = matStock
	uploadPlan.RequestPlan = plans
	uploadPlan.IsBom = true
	uploadPlan.IsCheckFg = true
	uploadPlan.IsUrgentByStockDif = false
	// uploadPlan.IsInitPlaned = true
	uploadPlan.IsNotInitPlaned = true

	_, err = CalculateUploadPlan(uploadPlan)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func GetStartCalDate(sqlx *sqlx.DB) time.Time {
	result, err := db.ExecuteQuery(sqlx, "select last_recal_date() date")

	if err != nil {
		return time.Now()
	}

	return result[0]["date"].(time.Time)
}

func getMaterialStock(sqlx *sqlx.DB, materialCodes []string) ([]MaterialStock, error) {
	matStock := []MaterialStock{}

	query := fmt.Sprintf(`
		select
			mat.material_code,
			coalesce(mat.current_qty , 0) qty
		from jit_master main
        left join materials mat on mat.material_id = main.fb_material_id
        where fg_material_id in (
        	select material_id from materials
            where
                is_deleted = false and
                inventory_mode = 3 and
                material_code in ('%s')
        ) and type = 1
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
