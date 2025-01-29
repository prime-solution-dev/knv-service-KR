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
	Description         any    `json:"description"`
	CurrentQty          any    `json:"currentQty"`
	LastTransactionDate any    `json:"lastTransactionDate"`
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
                                ,to_char(m.current_qty, 'fm999,999,999') qty
                        ,coalesce(greatest(last_gr, last_gi
                        )::varchar, '99') AS last_move_date
                from materials m
            where (current_date - greatest(last_gr, last_gi)) > %s
	`, req.OverDate)

	// fmt.Println(query)

	rows, err := db.ExecuteQuery(sqlx, query)
	if err != nil {
		return nil, err
	}

	for _, item := range rows {
		addItem := GetNonMoveMaterialResponse{
			Sku:                 item["sku"].(string),
			Description:         item["description"],
			CurrentQty:          item["qty"],
			LastTransactionDate: item["last_move_date"],
		}

		res = append(res, addItem)
	}

	return res, nil
}
