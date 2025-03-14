package jitInboundService

import (
	"encoding/json"
	"errors"
	"fmt"
	"jnv-jit/internal/cronjob"
	"jnv-jit/internal/db"
	"jnv-jit/internal/services/sftpService"
	"jnv-jit/internal/utils"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
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
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	} //todo: wait refactor to use reuseable code.

	cronjob.RegisterJob("upload-pipeline-kr-sun", GetFilesKr, `0 18 * * 0`)
	cronjob.RegisterJob("upload-pipeline-kr-tue", GetFilesKr, `0 4 * * 2`)
	cronjob.RegisterJob("upload-pipeline-kr-thu", GetFilesKr, `0 4 * * 4`)
	// cronjob.RegisterJob("upload-pipeline-kr-thu", UploadPlanPipelineKrCron, `*/1 * * * *`)
	println("register kr jobs successfully...")
}

func GetFilesKr() {
	filePath := os.Getenv("kr_file_path")
	processGetFiles(filePath, "LX02_", "ZM35_")
}

func UploadPlanPipelineKrCron() {
	sqlx, _ := db.ConnectSqlx(`jit_portal`)
	defer sqlx.Close()

	filePath := os.Getenv("kr_file_path")
	startCalDateFromDb := GetStartCalDateKr(sqlx).Truncate(24 * time.Hour)
	startFileDate := startCalDateFromDb
	startCalDate := startCalDateFromDb
	stockPath := filePath
	stockPrefixFile := `LX02_`
	planPath := filePath
	planPrefixFile := `ZM35_`

	// err := processGetFiles(filePath)
	// if err != nil {
	// 	println("can't get file")
	// } else {
	// 	ProcessUploadPipelineKr(startFileDate, startCalDate, stockPath, stockPrefixFile, planPath, planPrefixFile)
	// }

	ProcessUploadPipelineKr(startFileDate, startCalDate, stockPath, stockPrefixFile, planPath, planPrefixFile)
}

func processGetFiles(downloadPath string, lx02Prefix, zm35Prefix string) error {
	lx02Path := os.Getenv("lx02_path")
	zm35Path := os.Getenv("zm35_path")

	client, sshConn, err := sftpService.NewClient()
	if err != nil {
		return err
	}
	defer client.Close()
	defer sshConn.Close()

	lx02Files, err := client.ReadDir(lx02Path)
	if err != nil {
		return err
	}

	lx02LatestFile, err := utils.GetLatestFile(lx02Files, lx02Prefix)
	if err != nil {
		return err
	}

	zm35Files, err := client.ReadDir(zm35Path)
	if err != nil {
		return err
	}

	zm35LatestFile, err := utils.GetLatestFile(zm35Files, zm35Prefix)
	if err != nil {
		return err
	}

	downloadFunc := func(dstPath string, remotePath string) error {
		remoteFile, err := client.Open(remotePath)
		if err != nil {
			return err
		}

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		if _, err := remoteFile.WriteTo(dstFile); err != nil {
			return err
		}

		return nil
	}

	lx02LocalFilePath := filepath.Join(downloadPath, lx02LatestFile.Name())
	lx02RemoteFilePath := lx02Path + "/" + lx02LatestFile.Name()
	err = downloadFunc(lx02LocalFilePath, lx02RemoteFilePath)
	if err != nil {
		return err
	}

	zm35LocalFilePath := filepath.Join(downloadPath, zm35LatestFile.Name())
	zm35RemoteFilePath := zm35Path + "/" + zm35LatestFile.Name()
	err = downloadFunc(zm35LocalFilePath, zm35RemoteFilePath)
	if err != nil {
		return err
	}

	return nil
}

func ManualKrPipeline(c *gin.Context, jsonPayload string) (interface{}, error) {
	sqlx, _ := db.ConnectSqlx(`jit_portal`)
	defer sqlx.Close()

	filePath := os.Getenv("kr_file_path")
	// startFileDate := time.Now().Truncate(24 * time.Hour)
	startCalDateFromDb := GetStartCalDateKr(sqlx).Truncate(24 * time.Hour)
	startFileDate := startCalDateFromDb
	startCalDate := startCalDateFromDb
	stockPath := filePath
	stockPrefixFile := `LX02_`
	planPath := filePath
	planPrefixFile := `ZM35_`

	err := processGetFiles(filePath, stockPrefixFile, planPrefixFile)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("can't get file : %w", err))
	} else {
		if err := ProcessUploadPipelineKr(startFileDate, startCalDate, stockPath, stockPrefixFile, planPath, planPrefixFile); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func GetStartCalDateKr(sqlx *sqlx.DB) time.Time {
	result, err := db.ExecuteQuery(sqlx, "select start_cal_date_kr() date")

	if err != nil {
		return time.Now()
	}

	return result[0]["date"].(time.Time)
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

func ClearStock(gorm *gorm.DB) error {
	tx := gorm.Begin()

	tx.Table("materials").Where("is_deleted = false").Updates(map[string]any{
		"current_qty":  0,
		"updated_date": time.Now().Format(time.DateTime),
	})

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func GetNotExistsPlans(matNotExists []string, sqlx *sqlx.DB, startDate time.Time) ([]RequestPlan, error) {
	reqPlans := []RequestPlan{}

	qDate := startDate.Format(time.DateOnly)

	sql := fmt.Sprintf(`select
			mat.material_code,
			coalesce(hd.line_header_name, '') line_code
		from jit_daily main
		left join materials mat on mat.material_id = main.material_id
		left join jit_line_headers hd on hd.line_header_id = main.line_id
		where
			main.is_deleted = false and
			not exists(
				select 1 from jit_daily jd
				left join materials mat2 on mat2.material_id = jd.material_id
				where jd.is_deleted = false and mat2.material_code in ('%s') and
					main.material_id = jd.material_id
			) and
			(main.conf_date >= '%s' or main.conf_urgent_date >= '%s') and
			(main.conf_qty > 0 or main.conf_urgent_qty > 0)
		group by mat.material_code, hd.line_header_name
	`, strings.Join(matNotExists, "','"), qDate, qDate)

	items, err := db.ExecuteQuery(sqlx, sql)

	println(sql)

	if err != nil {
		return nil, err
	}

	for _, item := range items {
		reqPlans = append(reqPlans, RequestPlan{
			MaterialCode: item["material_code"].(string),
			LineCode:     item["line_code"].(string),
		})
	}

	return reqPlans, nil
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

	if stockFilePath == "" || planFilePath == "" {
		return nil
	}

	backup_file_path := os.Getenv("kr_file_path")

	exec.Command("mv", fmt.Sprintf("%s/**", stockPath), backup_file_path).Output()

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
	mats := []string{}
	matExists := make(map[string]bool)
	for _, item := range planMap {
		if exists := matExists[item.MaterialCode]; !exists {
			mats = append(mats, item.MaterialCode)
			matExists[item.MaterialCode] = true
		}
		plans = append(plans, item)
	}

	gormx, _ := db.ConnectGORM(`jit_portal`)
	defer db.CloseGORM(gormx)

	sqlx, _ := db.ConnectSqlx(`jit_portal`)
	defer sqlx.Close()

	notExistsPlans, err := GetNotExistsPlans(mats, sqlx, startCalDate)
	if err != nil {
		return nil
	}

	plans = append(plans, notExistsPlans...)

	ClearStock(gormx)

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
	uploadPlan.StartCal = startCalDate
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
		numColumns := len(data)
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
