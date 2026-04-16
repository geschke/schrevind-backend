package controller

import "strings"

func normalizeUserLocaleForController(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return "en-US"
	}
	return locale
}
