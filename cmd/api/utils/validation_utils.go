package utils

import (
	"regexp"
	"strconv"
)

func IsPositiveInteger(value string) bool {
	matched, _ := regexp.MatchString(`^[0-9]+$`, value)
	return matched
}

func ParseInt64(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}
