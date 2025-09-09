package jitInboundService

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

func TestTime(c *gin.Context, jsonPayload string) (interface{}, error) {

	now := time.Now()
	//now = time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, time.Local)

	fmt.Println("Now:", now)

	truncDate := now.Truncate(24 * time.Hour)
	fmt.Println("TruncDate: ", truncDate)

	setDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	fmt.Println("SetDate: ", setDate)

	setDate = setDate.Truncate(24 * time.Hour)
	fmt.Println("SetDateTrunc: ", setDate)

	return nil, nil
}
