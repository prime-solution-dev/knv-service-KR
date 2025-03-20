package reportService

import (
	"encoding/json"
	"errors"
	"fmt"
	"jnv-jit/internal/db"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type GetDashboardOverallRequest struct {
	Suppliers []string `json:"suppliers"`
	Materials []string `json:"materials"`
	UserId    int      `json:"userId"`
}

type GetDashboardOverallResponse struct {
	TotalRequest                 int   `json:"totalRequest"`
	TotalRequired                int   `json:"totalRequired"`
	TotalUrgent                  int   `json:"totalUrgent"`
	TotalRequiredMatch           int   `json:"totalRequiredMatch"`
	TotalUrgentMatch             int   `json:"totalUrgentMatch"`
	TotalRequiredMissmatch       int   `json:"totalRequiredMissmatch"`
	TotalUrgentMissmatch         int   `json:"totalUrgentMissmatch"`
	RequiredItems                []int `json:"requiredItems"`
	UrgentRequiredItems          []int `json:"urgentRequiredItems"`
	MissmatchRequiredItems       []int `json:"missmatchRequiredItems"`
	MissmatchUrgentRequiredItems []int `json:"missmatchUrgentRequiredItems"`
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
		// qCondSupplier = fmt.Sprintf(` and s.supplier_id in ('%s') `, strings.Join(req.Suppliers, `','`))
	}

	if len(req.Materials) > 0 {
		qCondMaterial = fmt.Sprintf(` and jd.material_id in (%s) `, strings.Join(req.Materials, `,`))
	}

	qCondUser := fmt.Sprintf(` and case when util_is_admin(%d) then true else 
        case
			when (select util_is_supplier(%d)) then
				mat.supplier_id = (select supplier_id from users where user_id = %d)
			else
				case when util_is_planner(%d) then
					(select planner_code from users where user_id = %d) like '%%' || mat.planner_code || '%%'
				else
					false
				end
			end
    	end`, req.UserId, req.UserId, req.UserId, req.UserId, req.UserId)

	startDate := time.Now().Format("2006-01-02")

	queryDaily := fmt.Sprintf(`
		select jd.jit_daily_id as request_id
			, jd.material_id 
			, jd.daily_date as request_date
			
			--Require
			, coalesce(jd.required_qty,0) as require_qty
			, coalesce(jd.conf_qty,0) as confirm_require_qty
			, coalesce(jd.conf_date, '1900-01-01') as confirm_require_date
			, case when jd_prd_rq.jit_daily_id is null then jd.daily_date else jd_prd_rq.daily_date end as production_require_date
			, jd_prd_rq.jit_daily_id as production_require_id
			
			--Urgent
			, coalesce(jd.urgent_qty,0) as urgent_qty
			, coalesce(jd.conf_urgent_qty,0) as confirm_urgent_qty
			, coalesce(jd.conf_urgent_date, '1900-01-01') as confirm_urgent_date
			, case when jd_prd_ur.jit_daily_id is null then jd.daily_date else jd_prd_ur.daily_date end as production_urgent_date
			, jd_prd_ur.jit_daily_id as production_urgent_id
			
		from jit_daily jd
		left join materials mat on mat.material_id = jd.material_id
		left join jit_daily jd_prd_rq on jd.original_jit_daily_id = jd_prd_rq.jit_daily_id 
		left join jit_daily jd_prd_ur on jd.original_jit_daily_id = jd_prd_ur.jit_daily_id 
		left join suppliers s on s.supplier_id = jd.supplier_id
		where 1=1 and jd.is_deleted = false 
		and jd.daily_date >= '%s'
		and (coalesce (jd.required_qty , 0) <> 0 or coalesce (jd.urgent_qty , 0) <> 0)
		%s
		%s
		%s
	`, startDate, qCondUser, qCondSupplier, qCondMaterial)
	println(queryDaily)

	rowsDaily, err := db.ExecuteQuery(sqlx, queryDaily)
	if err != nil {
		return nil, nil
	}

	if len(rowsDaily) == 0 {
		return nil, nil
	}

	for _, item := range rowsDaily {
		requestID := int(item["request_id"].(int64))
		productionRequireDate := item["production_require_date"].(time.Time)
		requireQty := item["require_qty"].(float64)
		confirmRequireDate := item["confirm_require_date"].(time.Time)
		confirmRequireQty := item["confirm_require_qty"].(float64)
		productionUrgentDate := item["production_urgent_date"].(time.Time)
		urgentQty := item["urgent_qty"].(float64)
		confirmUrgentDate := item["confirm_urgent_date"].(time.Time)
		confirmUrgentQty := item["confirm_urgent_qty"].(float64)

		if requireQty > 0 {
			if confirmRequireQty-requireQty >= 0 && !confirmRequireDate.After(productionRequireDate) {
				res.TotalRequiredMatch++
			} else {
				res.MissmatchRequiredItems = append(res.MissmatchRequiredItems, requestID)
				res.TotalRequiredMissmatch++
			}

			res.RequiredItems = append(res.RequiredItems, requestID)
			res.TotalRequired++
			res.TotalRequest++
		}

		if urgentQty > 0 {
			if confirmUrgentQty-urgentQty >= 0 && !confirmUrgentDate.After(productionUrgentDate) {
				res.TotalUrgentMatch++
			} else {
				res.MissmatchUrgentRequiredItems = append(res.MissmatchUrgentRequiredItems, requestID)
				res.TotalUrgentMissmatch++
			}

			res.UrgentRequiredItems = append(res.UrgentRequiredItems, requestID)
			res.TotalUrgent++
			res.TotalRequest++
		}
	}

	return res, nil
}
