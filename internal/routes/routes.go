package routes

import (
	"jnv-jit/internal/middlewares"
	confirmservice "jnv-jit/internal/services/confirm-service"
	inventoryService "jnv-jit/internal/services/inventory-service"
	jitInboundService "jnv-jit/internal/services/jit-inbound-service"
	"jnv-jit/internal/services/kanbanService"
	"jnv-jit/internal/services/reportService"
	testservice "jnv-jit/internal/services/testService"
	"jnv-jit/internal/utils"

	"github.com/gin-gonic/gin"
)

func init() {
}

func RegisterRoutes(router *gin.Engine) {
	router.Use(middlewares.CorsMiddleware())

	router.POST("/JIT/GetDashboardOverall", func(c *gin.Context) {
		utils.ProcessRequestPayload(c, reportService.GetDashboardOverall)
	})

	router.POST("/JIT/GetNonMoveMaterial", func(c *gin.Context) {
		utils.ProcessRequestPayload(c, reportService.GetNonMoveMaterial)
	})

	router.POST("/JIT/GetDashboardSummary", func(c *gin.Context) {
		utils.ProcessRequestPayload(c, reportService.GetDashboardSummary)
	})

	router.POST("/JITKanban/GetDashboardSummary", func(c *gin.Context) {
		utils.ProcessRequestPayload(c, kanbanService.GetDashboardSummary)
	})

	router.POST("/Test/ExtractUpdates", func(c *gin.Context) {
		utils.ProcessRequestPayload(c, testservice.TestExtractUpdates)
	})

	router.POST("/Inventory/UpdateInventory", func(c *gin.Context) {
		utils.ProcessRequestPayload(c, inventoryService.UpdateInventory)
	})

	router.POST("/JITInbound/UploadPipelineKr", func(c *gin.Context) {
		utils.ProcessRequestPayload(c, jitInboundService.UploadPlanPipelineKr)
	})

	router.POST("/JITInbound/UploadPlan", func(c *gin.Context) {
		utils.ProcessRequestMultiPart(c, jitInboundService.UploadPlan)
	})

	router.POST("/JITInbound/Confirm", func(c *gin.Context) {
		utils.ProcessRequestPayload(c, confirmservice.Confirm)
	})

	router.GET("/JITInbound/recal-lx02", func(c *gin.Context) {
		utils.ProcessRequestPayload(c, jitInboundService.RecalLx02)
	})

	router.GET("/JITInbound/manual-kr-pipeline", func(c *gin.Context) {
		utils.ProcessRequestPayload(c, jitInboundService.ManualKrPipeline)
	})

}
