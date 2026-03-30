package web

import (
	"context"
	"errors"
	"html"
	"net/url"
	"regexp"
	"strings"
)

const (
	defaultSearchLimit     = 5
	maxSearchLimit         = 10
	maxFetchedContentRunes = 4000
)

var (
	tagPattern           = regexp.MustCompile(`(?s)<[^>]+>`)
	htmlCommentPattern   = regexp.MustCompile(`(?is)<!--.*?-->`)
	scriptBlockPattern   = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleBlockPattern    = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	noscriptBlockPattern = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
	templateBlockPattern = regexp.MustCompile(`(?is)<template[^>]*>.*?</template>`)
	svgBlockPattern      = regexp.MustCompile(`(?is)<svg[^>]*>.*?</svg>`)
	headerBlockPattern   = regexp.MustCompile(`(?is)<header[^>]*>.*?</header>`)
	footerBlockPattern   = regexp.MustCompile(`(?is)<footer[^>]*>.*?</footer>`)
	navBlockPattern      = regexp.MustCompile(`(?is)<nav[^>]*>.*?</nav>`)
	asideBlockPattern    = regexp.MustCompile(`(?is)<aside[^>]*>.*?</aside>`)
	formBlockPattern     = regexp.MustCompile(`(?is)<form[^>]*>.*?</form>`)
	mainPattern          = regexp.MustCompile(`(?is)<main[^>]*>(.*?)</main>`)
	articlePattern       = regexp.MustCompile(`(?is)<article[^>]*>(.*?)</article>`)
	bodyPattern          = regexp.MustCompile(`(?is)<body[^>]*>(.*?)</body>`)
)

var errTavilyNoResults = errors.New("tavily returned no usable results")

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

func stripNoiseBlocks(input string) string {
	cleaned := htmlCommentPattern.ReplaceAllString(input, " ")
	for _, pattern := range []*regexp.Regexp{
		scriptBlockPattern,
		styleBlockPattern,
		noscriptBlockPattern,
		templateBlockPattern,
		svgBlockPattern,
		headerBlockPattern,
		footerBlockPattern,
		navBlockPattern,
		asideBlockPattern,
		formBlockPattern,
	} {
		cleaned = pattern.ReplaceAllString(cleaned, " ")
	}
	return cleaned
}

func extractMeaningfulContent(input string) string {
	cleaned := stripNoiseBlocks(input)
	for _, pattern := range []*regexp.Regexp{mainPattern, articlePattern, bodyPattern} {
		if match := pattern.FindStringSubmatch(cleaned); len(match) >= 2 {
			content := stripTags(match[1])
			if content != "" {
				return content
			}
		}
	}
	return stripTags(cleaned)
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

func shouldFallbackFromTavily(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errTavilyNoResults) {
		return true
	}

	var httpErr *tavilyHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == 429 || httpErr.StatusCode >= 500
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isTavilyTransportError(urlErr.Err)
	}

	return isTavilyTransportError(err)
}
