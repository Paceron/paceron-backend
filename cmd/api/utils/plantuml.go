package utils

import "encoding/hex"

func EncodePlantUML(source string) string {
	return hex.EncodeToString([]byte(source))
}
