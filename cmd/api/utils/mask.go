package utils

import "fmt"

// MaskSecret ofusca un valor sensible en logs: muestra solo los últimos 4
// caracteres con el largo total, para que sea trazable sin exponer el secreto.
func MaskSecret(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "***"
	}
	return "****" + value[len(value)-4:] + fmt.Sprintf("(len=%d)", len(value))
}
