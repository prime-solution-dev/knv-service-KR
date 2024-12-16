package utils

import "time"

func GetShortWeekday(date time.Time) string {
	return date.Weekday().String()[:3]
}
