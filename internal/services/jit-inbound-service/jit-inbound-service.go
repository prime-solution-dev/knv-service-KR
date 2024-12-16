package jitInboundService

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func FindLatestFileWithPrefix(dir string, prefix string) (string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("unable to read directory: %w", err)
	}

	var latestFile string
	var latestModTime time.Time

	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), prefix) {
			filePath := filepath.Join(dir, file.Name())

			info, err := file.Info()
			if err != nil {
				return "", fmt.Errorf("unable to stat file: %w", err)
			}

			if info.ModTime().After(latestModTime) {
				latestModTime = info.ModTime()
				latestFile = filePath
			}
		}
	}

	if latestFile == "" {
		return "", fmt.Errorf("no files found with the specified prefix in the directory")
	}

	return latestFile, nil
}

func ReadCsvFile(filePath string) ([]map[string]interface{}, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	reader.Comma = '\t'

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("unable to read CSV file: %w", err)
	}

	if len(rows) < 2 {
		return nil, errors.New("no data found in the CSV file")
	}

	headerStr := rows[0]
	headers := strings.Split(headerStr[0], ",")
	for i, h := range headers {
		headers[i] = strings.TrimSpace(h)
		if headers[i] == "" {
			headers[i] = fmt.Sprintf("Column%d", i+1)
		}
	}

	var results []map[string]interface{}

	for _, rowData := range rows[1:] {
		record := make(map[string]interface{})

		row := strings.Split(rowData[0], ",")

		for j := range headers {
			columnName := headers[j]

			if j >= len(row) || strings.TrimSpace(row[j]) == "" {
				record[columnName] = ""
			} else {
				record[columnName] = strings.TrimSpace(row[j])
			}
		}

		results = append(results, record)
	}

	return results, nil
}

func ReadPlainText(filePath string) ([]map[string]interface{}, error) {
	var data []map[string]interface{}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	rowIndex := 1
	for scanner.Scan() {
		if rowIndex == 1 {
			rowIndex++
			continue
		}
		line := scanner.Text()
		fields := strings.Split(strings.TrimSpace(line), "	")
		rowData := make(map[string]interface{})
		for j, field := range fields {
			colName := fmt.Sprintf("Col%d", j+1)
			rowData[colName] = strings.TrimSpace(field)
		}
		data = append(data, rowData)
		rowIndex++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file as text: %w", err)
	}

	return data, nil
}
