package prompt

const (
	maxPromptExcerptRunes = 2000
	headExcerptRunes      = 1200
	tailExcerptRunes      = 500
)

func summarizeToolResultForPrompt(toolName string, toolResult map[string]any) map[string]any {
	switch toolName {
	case "fs.read_file":
		return summarizeReadFileResult(toolResult)
	case "web.fetch":
		return summarizeWebFetchResult(toolResult)
	case "bash.exec":
		return summarizeBashExecResult(toolResult)
	default:
		return toolResult
	}
}

func summarizeReadFileResult(toolResult map[string]any) map[string]any {
	summary := map[string]any{}
	copyIfPresent(summary, toolResult, "path", "bytes")

	if content, ok := stringValue(toolResult["content"]); ok {
		excerpt, truncated := excerptText(content)
		summary["content_excerpt"] = excerpt
		summary["truncated"] = truthy(toolResult["truncated"]) || truncated
	}

	if _, ok := summary["truncated"]; !ok {
		if truncated, ok := boolValue(toolResult["truncated"]); ok {
			summary["truncated"] = truncated
		}
	}

	return summary
}

func summarizeWebFetchResult(toolResult map[string]any) map[string]any {
	summary := map[string]any{}
	copyIfPresent(summary, toolResult, "url", "final_url", "status_code", "title")

	if content, ok := stringValue(toolResult["content"]); ok {
		excerpt, truncated := excerptText(content)
		summary["content_excerpt"] = excerpt
		summary["truncated"] = truthy(toolResult["truncated"]) || truncated
	}

	if _, ok := summary["truncated"]; !ok {
		if truncated, ok := boolValue(toolResult["truncated"]); ok {
			summary["truncated"] = truncated
		}
	}

	return summary
}

func summarizeBashExecResult(toolResult map[string]any) map[string]any {
	summary := map[string]any{}
	copyIfPresent(summary, toolResult, "command", "workdir", "exit_code", "timed_out")

	stdoutExcerpt, stdoutTruncated := excerptText(mustString(toolResult["stdout"]))
	stderrExcerpt, stderrTruncated := excerptText(mustString(toolResult["stderr"]))
	summary["stdout_excerpt"] = stdoutExcerpt
	summary["stderr_excerpt"] = stderrExcerpt
	summary["truncated"] = truthy(toolResult["truncated"]) || stdoutTruncated || stderrTruncated
	return summary
}

func excerptText(text string) (string, bool) {
	runes := []rune(text)
	if len(runes) <= maxPromptExcerptRunes {
		return text, false
	}
	head := string(runes[:headExcerptRunes])
	tail := string(runes[len(runes)-tailExcerptRunes:])
	return head + "\n...\n" + tail, true
}

func copyIfPresent(dst, src map[string]any, keys ...string) {
	for _, key := range keys {
		if value, ok := src[key]; ok {
			dst[key] = value
		}
	}
}

func stringValue(value any) (string, bool) {
	s, ok := value.(string)
	return s, ok
}

func mustString(value any) string {
	s, _ := value.(string)
	return s
}

func boolValue(value any) (bool, bool) {
	b, ok := value.(bool)
	return b, ok
}

func truthy(value any) bool {
	b, ok := boolValue(value)
	return ok && b
}
