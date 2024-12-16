package kanbanService

import (
	"encoding/json"
	"errors"
	"fmt"
	"jnv-jit/internal/db"
	"strings"

	"github.com/gin-gonic/gin"
)

type GetDashboardOverallRequest struct {
	Suppliers []string `json:"suppliers"`
	Materials []string `json:"materials"`
}

type GetDashboardOverallResponse struct {
	TotalSku       int `json:"totalSku"`
	TotalGreen     int `json:"totalGreen"`
	TotalYellow    int `json:"totalYellow"`
	TotalRed       int `json:"totalRed"`
	TotalOvergreen int `json:"totalOvergreen"`
}

func GetDashboardOverall(c *gin.Context, jsonPayload string) (interface{}, error) {
	var req GetDashboardOverallRequest
	var res GetDashboardOverallResponse

	if err := json.Unmarshal([]byte(jsonPayload), &req); err != nil {
		return nil, errors.New("failed to unmarshal JSON into struct: " + err.Error())
	}

	sqlx, err := db.ConnectSqlx(`jit_portal`)
	if err != nil {
		return nil, err
	}
	defer sqlx.Close()

	qCondSupplier := ``
	qCondMaterial := ``

	if len(req.Suppliers) > 0 {
		qCondSupplier = fmt.Sprintf(` and s.supplier_code in ('%s') `, strings.Join(req.Suppliers, `','`))
	}

	if len(req.Materials) > 0 {
		qCondMaterial = fmt.Sprintf(` and m.material_code in ('%s')`, strings.Join(req.Materials, `','`))
	}

	queryKanban := fmt.Sprintf(`
		select kp.kanban_progress_id 
				--, case when kp.yellow_date is not null then 'YELLOW' else 'RED' end request_type
				, kp.status as kanban_status
		from kanban_progress kp
		left join materials m on m.material_id = kp.material_id
		left join suppliers s on m.supplier_id = s.supplier_id 
		where 1=1
		and	(kp.yellow_date >= '2024-11-1' or kp.red_date >= '2024-11-1')
		%s
		%s
	`, qCondSupplier, qCondMaterial)
	rowsKanban, err := db.ExecuteQuery(sqlx, queryKanban)
	if err != nil {
		return nil, err
	}

	for _, item := range rowsKanban {
		statusKanban := int(item["kanban_status"].(float64))

		res.TotalSku++

		if statusKanban == 1 {
			res.TotalGreen++
		} else if statusKanban == 2 {
			res.TotalYellow++
		} else if statusKanban == 3 {
			res.TotalRed++
		} else if statusKanban == 4 {
			res.TotalOvergreen++
		}
	}

	return res, nil
}
