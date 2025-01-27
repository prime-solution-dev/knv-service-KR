package utils

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

func ReadExcelFile(c *gin.Context, formFieldName, sheetName string) ([]map[string]interface{}, string, error) {
	file, err := c.FormFile(formFieldName)
	fileName := file.Filename

	if err != nil {
		return nil, fileName, fmt.Errorf("file upload error: %w", err)
	}

	f, err := file.Open()
	if err != nil {
		return nil, fileName, fmt.Errorf("unable to open file: %w", err)
	}
	defer f.Close()

	xlsx, err := excelize.OpenReader(f)
	if err != nil {
		return nil, fileName, fmt.Errorf("unable to read Excel file: %w", err)
	}

	if sheetName == "" {
		sheets := xlsx.GetSheetList()
		if len(sheets) == 0 {
			return nil, fileName, errors.New("no sheet found in the Excel file")
		}
		sheetName = sheets[0]
	}

	rows, err := xlsx.GetRows(sheetName)
	if err != nil {
		return nil, fileName, fmt.Errorf("unable to read rows from sheet: %w", err)
	}

	if len(rows) < 2 {
		return nil, fileName, errors.New("no data found in the Excel file")
	}

	headers := rows[0]
	for i, h := range headers {
		headers[i] = strings.TrimSpace(h)
	}

	var results []map[string]interface{}

	for _, row := range rows[1:] {
		record := make(map[string]interface{})
		for j := range headers {
			columnName := headers[j]

			if j >= len(row) || strings.TrimSpace(row[j]) == "" {
				record[columnName] = nil
			} else {
				cell := row[j]
				var v interface{}
				switch val := interface{}(cell).(type) {
				case string:
					if f, err := strconv.ParseFloat(val, 64); err == nil {
						v = f
					} else {
						v = val
					}
				case []byte:
					strVal := string(val)
					if f, err := strconv.ParseFloat(strVal, 64); err == nil {
						v = f
					} else {
						v = strVal
					}
				case float64:
					v = float64(val)
				case int32:
					v = int(val)
				case int64:
					v = int64(val)
				default:
					v = val
				}

				record[columnName] = v
			}
		}
		results = append(results, record)
	}

	return results, fileName, nil
}
