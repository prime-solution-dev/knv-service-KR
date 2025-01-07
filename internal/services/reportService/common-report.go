package reportService

import (
	"time"
)

func GetMondayDateOfCurrentWeek() time.Time {
	now := time.Now()

	offset := int(time.Monday - now.Weekday())
	if offset > 0 {
		offset = -6
	}

	return now.AddDate(0, 0, offset)
}

func GetMondayOfCurrentWeek() string {
	return GetMondayDateOfCurrentWeek().Format("2006-01-02")
}

func GetStartRecal() {
	//Todo function

	// sqlx, err := db.ConnectSqlx(`jit_portal_kr`)
	// if err != nil {
	// 	return nil, err
	// }
	// defer sqlx.Close()

	// query := `

	// `
	// rows, err := db.ExecuteQuery(sqlx, query)
}

func GenerateBackWeeks(startDate time.Time, numWeeks int) []Week {
	var weeks []Week

	startOfWeek := startDate.Truncate(24 * time.Hour)
	for startOfWeek.Weekday() != time.Monday {
		startOfWeek = startOfWeek.AddDate(0, 0, -1)
	}

	for i := 0; i <= numWeeks; i++ {
		endOfWeek := startOfWeek.AddDate(0, 0, 6)

		_, weekNumber := startOfWeek.ISOWeek()

		weeks = append(weeks, Week{
			WeekNumber:  weekNumber,
			StartOfWeek: startOfWeek,
			EndOfWeek:   endOfWeek,
		})

		startOfWeek = startOfWeek.AddDate(0, 0, -7)
	}

	return weeks
}

func GetEarliestMonday(startDate time.Time, numWeeks int) time.Time {
	weeks := GenerateBackWeeks(startDate, numWeeks)
	if len(weeks) > 0 {
		return weeks[len(weeks)-1].StartOfWeek
	}
	return time.Time{}
}

func ConvertDateToDay(dateStr string) (string, error) {
	layout := "2006-01-02"

	t, err := time.Parse(layout, dateStr)
	if err != nil {
		return "", err
	}

	day := t.Weekday().String()[:3]

	return day, nil
}

func isDateInRange(requireDate, confirmDate time.Time) (bool, error) {
	startRange := requireDate.AddDate(0, 0, -3)
	endRange := requireDate.AddDate(0, 0, 1)

	if (confirmDate.After(startRange) && confirmDate.Before(endRange)) || confirmDate.Equal(startRange) || confirmDate.Equal(endRange) {
		return true, nil
	}

	return false, nil
}
