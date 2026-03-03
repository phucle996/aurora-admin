package service

import "time"

func formatTimeLocal(t time.Time) string {
	return t.In(time.Local).Format(time.RFC3339)
}
