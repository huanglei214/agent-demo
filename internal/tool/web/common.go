package web

import (
	"html"
	"regexp"
	"strings"
)

const (
	defaultSearchLimit     = 5
	maxSearchLimit         = 10
	maxFetchedContentRunes = 4000
)

var tagPattern = regexp.MustCompile(`(?s)<[^>]+>`)

func normalizeLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultSearchLimit
	case limit > maxSearchLimit:
		return maxSearchLimit
	default:
		return limit
	}
}

func stripTags(input string) string {
	withoutTags := tagPattern.ReplaceAllString(input, " ")
	unescaped := html.UnescapeString(withoutTags)
	return strings.Join(strings.Fields(unescaped), " ")
}

func truncateString(input string, maxRunes int) (string, bool) {
	if maxRunes <= 0 {
		return input, false
	}
	runes := []rune(input)
	if len(runes) <= maxRunes {
		return input, false
	}
	return string(runes[:maxRunes]), true
}
