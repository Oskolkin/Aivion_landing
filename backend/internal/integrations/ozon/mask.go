package ozon

func MaskClientID(value string) string {
	if value == "" {
		return ""
	}

	if len(value) <= 4 {
		return "****"
	}

	if len(value) <= 6 {
		return value[:1] + "***" + value[len(value)-1:]
	}

	return value[:2] + "***" + value[len(value)-2:]
}
