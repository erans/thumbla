package utils

// SafeCastToString safely casts an interface to a string, otherwise it will return an empty string
func SafeCastToString(v interface{}) string {
	if v != nil {
		return v.(string)
	}

	return ""
}
