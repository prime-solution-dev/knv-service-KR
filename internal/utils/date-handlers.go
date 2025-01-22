package utils

import "time"

func GetShortWeekday(date time.Time) string {
	return date.Weekday().String()[:3]
}

func GetWeekendDate(date time.Time) time.Time {
	daysUntilSunday := (7 - int(date.Weekday())) % 7

	return date.AddDate(0, 0, daysUntilSunday)
}
