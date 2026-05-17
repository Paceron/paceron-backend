package utils

import "strconv"

func StringToInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func Int64ToString(i int64) string {
	return strconv.FormatInt(i, 10)
}
