package jitInboundService

import (
	"fmt"
	"jnv-jit/internal/db"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func getRecalMats(sqlx *sqlx.DB, startCal time.Time) []string {
	mats := []string{}

	query := fmt.Sprintf(`
		select distinct mat.material_code from jit_daily main
		left join materials mat on mat.material_id = main.material_id
		where main.is_deleted = false and
			main.daily_date >= '%s'
	`, startCal.Format(time.DateOnly))

	println(query)

	items, err := db.ExecuteQuery(sqlx, query)

	if err == nil {
		for _, data := range items {
			mats = append(mats, data["material_code"].(string))
		}
	}

	return mats
}

func getMaterialBomStock(sqlx *sqlx.DB, materialCodes []string) ([]MaterialStock, error) {
	matStock := []MaterialStock{}

	query := fmt.Sprintf(`
		select
			mat.material_code,
			coalesce(mat.current_qty , 0) qty
		from jit_master main
        left join materials mat on mat.material_id = main.fb_material_id
        where fb_material_id in (
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

func RecalLx02(c *gin.Context, jsonPayload string) (interface{}, error) {
	sqlx, err := db.ConnectSqlx(`jit_portal`)
	if err != nil {
		return nil, err
	}
	defer sqlx.Close()

	gormx, err := db.ConnectGORM(`jit_portal`)
	if err != nil {
		return nil, err
	}
	defer db.CloseGORM(gormx)

	startDate := GetStartCalDate(sqlx)

	bomMats := getRecalMats(sqlx, startDate)

	matStock, err := getMaterialStock(sqlx, bomMats, true)
	if err != nil {
		return nil, err
	}

	jitDailyDBMap, err := GetJitDailyDB(sqlx, startDate, bomMats)
	if err != nil {
		return nil, fmt.Errorf("error get jit daily db: %w", err)
	}

	jitDailyMap := make(map[string][]JitLine)

	jitDailyMap, err = MergeJitDaily(startDate, jitDailyMap, jitDailyDBMap, true)
	if err != nil {
		return nil, fmt.Errorf("error merge jit daily: %w", err)
	}

	jitMats, confirmMap, materialCodes, lineCodes, maxLineId, err := BuildToCalStruct(startDate, jitDailyMap, matStock)
	if err != nil {
		return nil, fmt.Errorf("error build cal struct: %w", err)
	}

	lastEndStockMat, err := GetLastEndStockMaterial(sqlx, materialCodes, startDate)
	if err != nil {
		return nil, fmt.Errorf("error get last end stock: %w", err)
	}

	jitMats, err = CalculateUrgentStockDiff(jitMats, lastEndStockMat, startDate)
	if err != nil {
		return nil, fmt.Errorf("error cal urgent stock diff: %w", err)
	}

	adjustLeadtimeMap := GetAdjustLeadtimeRequire(sqlx)

	materialMap, err := GetMatrialMap(sqlx, materialCodes)
	if err != nil {
		return nil, err
	}

	jitMats, err = CalculateEstimate(jitMats, adjustLeadtimeMap, maxLineId, materialMap)
	if err != nil {
		return nil, fmt.Errorf("error calculate estimate: %w", err)
	}

	jitMats, err = CalculateActual(jitMats, confirmMap)
	if err != nil {
		return nil, fmt.Errorf("error calculate actual: %w", err)
	}

	lineMap, err := GetLineMap(sqlx, lineCodes)
	if err != nil {
		return nil, err
	}

	jitDailys, err := ConvertToJitDailyDB(jitMats, lineMap, materialMap)
	if err != nil {
		return nil, fmt.Errorf("error convert jit daily db: %w", err)
	}

	materialIds := []int{}
	for _, mat := range materialMap {
		materialIds = append(materialIds, int(mat.MaterialId))
	}

	err = CreateJitDaily(gormx, sqlx, []JitProcess{}, jitDailys, startDate, true, materialIds)
	if err != nil {
		return nil, fmt.Errorf("error create jit daily: %w", err)
	}

	return nil, nil
}
