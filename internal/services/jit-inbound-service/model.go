package jitInboundService

import "time"

func (JitDaily) TableName() string {
	return "jit_daily"
}

func (JitProcess) TableName() string {
	return "jit_process"
}

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

type JitProcess struct {
	JitProcessID    int64      `gorm:"column:jit_process_id;primaryKey;autoIncrement"`
	LineID          int64      `gorm:"column:line_id"`
	FGMaterialID    int64      `gorm:"column:fg_material_id"`
	LineName        string     `gorm:"column:line_name"`
	FGCode          string     `gorm:"column:fg_code"`
	FGDescription   string     `gorm:"column:fg_description"`
	PlanQty         float64    `gorm:"column:plan_qty"`
	ProductSAP      string     `gorm:"column:product_sap"`
	StartTime       time.Time  `gorm:"column:start_time"`
	FinishTime      *time.Time `gorm:"column:finish_time"`
	TotalMinuteUsed int64      `gorm:"column:total_minute_used"`
	IsBreak         bool       `gorm:"column:is_break"`
	ImportID        int64      `gorm:"column:import_id"`
	IsDaily         bool       `gorm:"column:is_daily"`
	IsProcess       bool       `gorm:"column:is_process"`
	IsDeleted       bool       `gorm:"column:is_deleted"`
	UpdatedBy       int64      `gorm:"column:updated_by"`
	UpdatedDate     time.Time  `gorm:"column:updated_date"`
	CreatedBy       int64      `gorm:"column:created_by"`
	CreatedDate     time.Time  `gorm:"column:created_date"`
}

type JitMaterial struct {
	MaterialCode string
	LeadTime     int64
	Stock        MaterialStock
	JitDates     []JitDate
}

type JitDate struct {
	Date               time.Time
	BeginStock         float64
	PlantSock          float64
	SubconStock        float64
	ProductionQty      float64
	RequireQty         float64
	UrgentQty          float64
	Diff               float64
	EstimateStock      float64
	ConfirmQty         float64
	ConfirmRequireQty  float64
	ConfirmUrgentQty   float64
	ConfirmRequireDate *time.Time
	ConfirmUrgentDate  *time.Time
	EndStock           float64
	Lines              []JitLine
}

type JitLine struct {
	id                  int64
	PlanId              *int64
	RefReuqestID        *int64
	MaterialCode        string
	DailyDate           time.Time
	LineCode            string
	ProductionQty       float64
	ProductionPlantQty  float64
	ProductionSubconQty float64
	RequireQty          float64
	UrgenQty            float64
	ConfirmRequireQty   float64
	ConfirmUrgentQty    float64
	ConfirmRequireDate  *time.Time
	ConfirmUrgentDate   *time.Time
	LeadTime            int64
	IsStockDiff         bool
	IsStockDiffQty      float64
}

type JitDilyPlan struct {
	PlanId           int64
	MaterialCode     string
	LineCode         string
	RequestQty       float64
	RequestPlantQty  float64
	RequestSubconQty float64
	PlanDate         time.Time
	EndPlanDate      *time.Time
}

type Material struct {
	MaterialId    int64
	MaterialCode  string
	SupplierId    int64
	SupplierCode  string
	Qty           float64
	LeadTime      int64
	PalletPattern *float64
	Boms          []Bom
}

type Line struct {
	LineId   int64
	LineCode string
}

type Bom struct {
	MaterialId   int64
	MaterialCode string
	Qty          float64
	LeadTime     int64
	Waste        float64
}

type MaterialLine struct {
	MaterialCode string
	LineCode     string
}
