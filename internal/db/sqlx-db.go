package db

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func ConnectSqlx(databaseName string) (*sqlx.DB, error) {

	dabaseUrl := os.Getenv(fmt.Sprintf("database_sqlx_url_%s", databaseName))
	if dabaseUrl == `` {
		return nil, fmt.Errorf("not found database_sqlx_url")
	}

	db, err := sqlx.Connect("postgres", dabaseUrl)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	return db, nil
}

func ExecuteQuery(db *sqlx.DB, query string) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	rows, err := db.QueryxContext(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %v", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get column names: %v", err)
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]

			switch val := val.(type) {
			case []byte:
				strVal := string(val)
				if f, err := strconv.ParseFloat(strVal, 64); err == nil {
					v = f
				} else {
					if parsedUUID, err := uuid.FromBytes(val); err == nil {
						v = parsedUUID.String()
					} else {
						v = strVal
					}
				}
			case string:
				if parsedUUID, err := uuid.Parse(val); err == nil {
					v = parsedUUID.String()
				} else {
					v = val
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

			row[col] = v
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during iteration: %v", err)
	}

	return results, nil
}
