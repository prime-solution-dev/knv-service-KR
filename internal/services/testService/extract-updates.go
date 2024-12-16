package testservice

import (
	"encoding/json"
	"errors"
	"jnv-jit/internal/db"
	"jnv-jit/internal/utils"

	"github.com/gin-gonic/gin"
)

type CronJobs struct {
	JobName        string  `gorm:"column:job_name;primaryKey" json:"job_name"`
	CronExpression *string `gorm:"column:cron_expression" json:"cron_expression"`
	Desc           *string `gorm:"column:desc" json:"desc"`
}

func TestExtractUpdates(c *gin.Context, jsonPayload string) (interface{}, error) {
	var req []CronJobs

	if err := json.Unmarshal([]byte(jsonPayload), &req); err != nil {
		return nil, errors.New("failed to unmarshal JSON into struct: " + err.Error())
	}

	gormx, err := db.ConnectGORM(`pg_sale`)
	if err != nil {
		return nil, err
	}
	defer db.CloseGORM(gormx)

	updateDatas := []CronJobs{}

	for _, item := range req {
		updateDatas = append(updateDatas, CronJobs{
			JobName:        item.JobName,
			CronExpression: item.CronExpression,
			Desc:           item.Desc,
		})
	}

	updates, err := utils.ExtractUpdates([]interface{}{updateDatas})
	if err != nil {
		return nil, err
	}

	tx := gormx.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	for _, update := range updates {
		if jobName, exists := update["JobName"]; exists {
			if err = tx.Model(&CronJobs{}).
				Where("job_name = ?", jobName).
				Updates(update).Error; err != nil {
				return nil, err
			}
			// if err = tx.Debug().Model(&CronJobs{}).
			// 	Where("job_name = ?", jobName).
			// 	Updates(update).Error; err != nil {
			// 	return nil, err
			// }
		}
	}

	return nil, nil
}
