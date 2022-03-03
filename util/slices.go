package util

// ContainsString searches a given value in a string slice
func ContainsString(slice []string, value string) bool {
	for _, i := range slice {
		if i == value {
			return true
		}
	}
	return false
}
