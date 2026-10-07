package mapping

import "strings"

func validMode(mode string) bool {
	switch strings.ToUpper(mode) {
	case "ALT", "VS", "IAS", "HDG", "CRS":
		return true
	}
	return false
}
func validColor(color string) bool {
	switch color {
	case "green", "red", "yellow", "off":
		return true
	}
	return false
}
