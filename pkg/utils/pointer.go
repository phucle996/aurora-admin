package utils

func DerefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
