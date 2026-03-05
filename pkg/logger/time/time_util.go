package time_util

import "time"

func FormatTimeLocal(t time.Time) string {
	return t.In(time.Local).Format(time.RFC3339)
}
