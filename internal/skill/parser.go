package skill

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type frontmatter struct {
	Name          string
	Description   string
	Compatibility string
	AllowedTools  []string
	Tags          []string
}

func parseMetadata(path string, scope Scope) (Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Metadata{}, err
	}
	fm, _ := parseFrontmatter(string(data))
	dir := filepath.Dir(path)
	return Metadata{
		Name:          fm.Name,
		Description:   fm.Description,
		Compatibility: fm.Compatibility,
		AllowedTools:  normalizeList(fm.AllowedTools),
		Tags:          normalizeList(fm.Tags),
		Scope:         scope,
		Dir:           dir,
		EntryPath:     path,
	}, nil
}

func parseDefinition(path string, scope Scope) (Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, err
	}
	fm, body := parseFrontmatter(string(data))
	meta, err := parseMetadata(path, scope)
	if err != nil {
		return Definition{}, err
	}
	if strings.TrimSpace(meta.Name) == "" {
		meta.Name = filepath.Base(filepath.Dir(path))
	}
	if strings.TrimSpace(meta.Description) == "" {
		meta.Description = fm.Description
	}
	return Definition{
		Metadata:     meta,
		Instructions: strings.TrimSpace(body),
	}, nil
}

func parseFrontmatter(content string) (frontmatter, string) {
	trimmed := strings.TrimLeft(content, "\ufeff\r\n\t ")
	if !strings.HasPrefix(trimmed, "---\n") && !strings.HasPrefix(trimmed, "---\r\n") {
		return frontmatter{}, content
	}

	scanner := bufio.NewScanner(strings.NewReader(trimmed))
	lines := []string{}
	if !scanner.Scan() {
		return frontmatter{}, content
	}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		lines = append(lines, line)
	}

	fm := parseFrontmatterLines(lines)
	bodyStart := strings.Index(trimmed, strings.Join(append([]string{"---"}, lines...), "\n"))
	if bodyStart < 0 {
		return fm, ""
	}
	body := trimmed
	// Find second delimiter and everything after it.
	if idx := strings.Index(body[len("---"):], "\n---"); idx >= 0 {
		rest := body[len("---")+idx+len("\n---"):]
		rest = strings.TrimPrefix(rest, "\r")
		rest = strings.TrimPrefix(rest, "\n")
		return fm, rest
	}
	return fm, ""
}

func parseFrontmatterLines(lines []string) frontmatter {
	var (
		fm      frontmatter
		listKey string
	)
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") && listKey != "" {
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			switch listKey {
			case "allowed-tools":
				fm.AllowedTools = append(fm.AllowedTools, item)
			case "tags":
				fm.Tags = append(fm.Tags, item)
			}
			continue
		}
		listKey = ""
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		if value == "" {
			switch key {
			case "allowed-tools", "tags":
				listKey = key
			}
			continue
		}
		switch key {
		case "name":
			fm.Name = value
		case "description":
			fm.Description = value
		case "compatibility":
			fm.Compatibility = value
		case "allowed-tools":
			fm.AllowedTools = append(fm.AllowedTools, splitInlineList(value)...)
		case "tags":
			fm.Tags = append(fm.Tags, splitInlineList(value)...)
		}
	}
	return fm
}

func splitInlineList(value string) []string {
	value = strings.TrimSpace(strings.Trim(value, "[]"))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.Trim(strings.TrimSpace(part), `"'`)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func normalizeList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}
