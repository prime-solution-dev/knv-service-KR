package reportService

import (
	"encoding/json"
	"errors"
	"fmt"
	"jnv-jit/internal/db"

	"github.com/gin-gonic/gin"
)

type GetNonMoveMaterialRequest struct {
	OverDate int `json:"over_date"`
}

type GetNonMoveMaterialResponse struct {
	Sku                 string `json:"sku"`
	Description         string `json:"description"`
	CurrentQty          string `json:"currentQty"`
	LastTransactionDate string `json:"lastTransactionDate"`
}

func GetNonMoveMaterial(c *gin.Context, jsonPayload string) (interface{}, error) {
	var req GetNonMoveMaterialRequest
	var res []GetNonMoveMaterialResponse

	if err := json.Unmarshal([]byte(jsonPayload), &req); err != nil {
		return nil, errors.New("failed to unmarshal JSON into struct: " + err.Error())
	}

	sqlx, err := db.ConnectSqlx(`jit_portal`)
	if err != nil {
		return nil, err
	}
	defer sqlx.Close()

	if req.OverDate == 0 {
		return nil, errors.New(`require request body`)
	}

	query := fmt.Sprintf(`
		select m.material_code sku
				,m.description 
				,m.current_qty qty
			,greatest(
				coalesce(cast(last_gr as date), date '1900-01-01'), 
				coalesce(last_gi, date '1900-01-01')
			) AS last_move_date
		from materials m
		where (current_date - greatest(
				coalesce(cast(last_gr as date), date '1900-01-01'), 
				coalesce(last_gi, date '1900-01-01')
			)) > %d
		and (last_gr is not null OR last_gi is not null)
	`, req.OverDate)
	rows, err := db.ExecuteQuery(sqlx, query)
	if err != nil {
		return nil, err
	}

	for _, item := range rows {
		addItem := GetNonMoveMaterialResponse{
			Sku:                 item["sku"].(string),
			Description:         item["description"].(string),
			CurrentQty:          item["qty"].(string),
			LastTransactionDate: item["last_move_date"].(string),
		}

		res = append(res, addItem)
	}

	return res, nil
}
