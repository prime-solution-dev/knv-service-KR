package uploadlog

import (
	"fmt"
	"jnv-jit/internal/db"
	"time"

	"github.com/jmoiron/sqlx"
)

func AddUploadLog(sqlx *sqlx.DB, filename string, uploadRow int, uploadStatus bool, uploadReason string, uploadBy int) error {
	sql := fmt.Sprintf(`INSERT INTO upload_logs (
		master_name,
		type,
		file_name,
		upload_row,
		status,
		percent,
		import_date,
		last_update_date,
		upload_reason,
		action_by
	) VALUES (
		'%s', '%s', '%s', %d, %t, %d, '%s', '%s', '%s', %d
	)`,
		"jit-daily-confirm-delivery",
		"-",
		filename,
		uploadRow,
		uploadStatus,
		100,
		time.Now().Format(time.RFC3339),
		time.Now().Format(time.RFC3339),
		uploadReason, uploadBy)
	_, err := db.ExecuteQuery(sqlx, sql)

	return err
}
