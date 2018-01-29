package utils

import "math"

// SafeCastToString safely casts an interface to a string, otherwise it will return an empty string
func SafeCastToString(v interface{}) string {
	if v != nil {
		return v.(string)
	}

	return ""
}

// Round will round floats to the nearest value
func Round(val float64, roundOn float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}
