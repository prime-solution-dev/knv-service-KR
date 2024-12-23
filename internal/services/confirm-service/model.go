package confirmservice

import "time"

type JitDaily struct {
	JitDailyID         int64      `gorm:"column:jit_daily_id;primaryKey;autoIncrement"`
	JitDailyPlanID     int64      `gorm:"column:jit_daily_plan_id"`
	MaterialID         int64      `gorm:"column:material_id"`
	LineID             int64      `gorm:"column:line_id"`
	SupplierID         int64      `gorm:"column:supplier_id"`
	DailyDate          time.Time  `gorm:"column:daily_date"`
	ConfDate           *time.Time `gorm:"column:conf_date"`
	ConfUrgentDate     *time.Time `gorm:"column:conf_urgent_date"`
	BeginStock         float64    `gorm:"column:begin_stock"`
	PlantStock         float64    `gorm:"column:plant_stock"`
	SubconStock        float64    `gorm:"column:subcon_stock"`
	ProductQty         float64    `gorm:"column:product_qty"`
	PlantQty           float64    `gorm:"column:plant_qty"`
	SubconQty          float64    `gorm:"column:subcon_qty"`
	RequiredQty        float64    `gorm:"column:required_qty"`
	UrgentQty          float64    `gorm:"column:urgent_qty"`
	ConfQty            float64    `gorm:"column:conf_qty"`
	ConfUrgentQty      float64    `gorm:"column:conf_urgent_qty"`
	StockUpdate        float64    `gorm:"column:stock_update"`
	PlantUpdate        float64    `gorm:"column:plant_update"`
	SubconUpdate       float64    `gorm:"column:subcon_update"`
	CurrentStock       float64    `gorm:"column:current_stock"`
	EndOfStock         float64    `gorm:"column:end_of_stock"`
	PlantEndOfStock    float64    `gorm:"column:plant_end_of_stock"`
	SubconEndOfStock   float64    `gorm:"column:subcon_end_of_stock"`
	DailyStatus        int64      `gorm:"column:daily_status"`
	IsDeleted          bool       `gorm:"column:is_deleted"`
	UpdatedBy          int64      `gorm:"column:updated_by"`
	UpdatedDate        time.Time  `gorm:"column:updated_date"`
	CreatedBy          int64      `gorm:"column:created_by"`
	CreatedDate        time.Time  `gorm:"column:created_date"`
	ConfQtyKPI         float64    `gorm:"column:conf_qty_kpi"`
	DateConfKPI        *time.Time `gorm:"column:date_conf_kpi"`
	ActualQtyKPI       float64    `gorm:"column:actual_qty_kpi"`
	SummaryKPI         float64    `gorm:"column:summary_kpi"`
	UrgentConfQtyKPI   float64    `gorm:"column:urgent_conf_qty_kpi"`
	UrgentDateConfKPI  *time.Time `gorm:"column:urgent_date_conf_kpi"`
	UrgentActualQtyKPI *float64   `gorm:"column:urgent_actual_qty_kpi"`
	UrgentSummaryKPI   *float64   `gorm:"column:urgent_summary_kpi"`
	IsGenerate         *bool      `gorm:"column:is_generate"`
	DailyTime          *time.Time `gorm:"column:daily_time"`
	OriginalJitDailyID *int64     `gorm:"column:original_jit_daily_id"`
	StartCalRequired   *bool      `gorm:"column:start_cal_required"`
	StartCalUrgent     *bool      `gorm:"column:start_cal_urgent"`
	StartCalProd       *bool      `gorm:"column:start_cal_prod"`
	IsNewRequired      *bool      `gorm:"column:is_new_required"`
}

type ConfirmRequest struct {
	RowIndex     int        `json:"row_index"`
	DailyType    string     `json:"Daily Type"`
	Tech         string     `json:"Tech"`
	MaterialCode string     `json:"Material"`
	SupplierCode string     `json:"SupplierCode"`
	Description  string     `json:"Description"`
	RequiredQty  float64    `json:"Required QTY"`
	UrgentQty    float64    `json:"Urgent QTY"`
	RequiredDate *time.Time `json:"Req. Del Date"`
	ConfQty      float64    `json:"Conf. Del. QTY"`
	ConfDate     *time.Time `json:"Conf. Del. Date(MM/DD/YYYY)"`
}

type ConfirmData struct {
	MaterialId       int
	MaterialCode     string
	RequiredDate     time.Time
	RequireTime      time.Time
	DailyTime        string
	RequiredQty      float64
	UrgentQty        float64
	JitDailyID       int
	LineID           int
	ConfirmQty       float64
	ConfirmUrgentQty float64
	ConfirmDate      *time.Time
	UrgentDate       *time.Time
}

type ConfirmMinMatDate struct {
	MinDate   time.Time
	Materials string
}
