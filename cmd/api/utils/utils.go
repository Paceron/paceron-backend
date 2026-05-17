package utils

const (
	StringEmpty = ""
)

func Contains[T comparable](elementList []T, element T) bool {
	for _, v := range elementList {
		if v == element {
			return true
		}
	}
	return false
}
