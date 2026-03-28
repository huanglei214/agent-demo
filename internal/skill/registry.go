package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type Scope string

const (
	ScopeProject Scope = "project"
	ScopeUser    Scope = "user"
)

type Source struct {
	Scope Scope
	Root  string
}

type Metadata struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Compatibility string   `json:"compatibility,omitempty"`
	AllowedTools  []string `json:"allowed_tools,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Scope         Scope    `json:"scope"`
	Dir           string   `json:"dir"`
	EntryPath     string   `json:"entry_path"`
}

type Definition struct {
	Metadata
	Instructions string `json:"instructions"`
}

type Registry struct {
	sources []Source
}

func NewRegistry(workspace string) Registry {
	sources := []Source{
		{
			Scope: ScopeProject,
			Root:  filepath.Join(workspace, "skills"),
		},
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		sources = append(sources, Source{
			Scope: ScopeUser,
			Root:  filepath.Join(home, ".agent-demo", "skills"),
		})
	}
	return Registry{sources: sources}
}

func NewRegistryWithSources(sources []Source) Registry {
	return Registry{sources: append([]Source(nil), sources...)}
}

func (r Registry) List() ([]Metadata, error) {
	index, err := r.buildIndex()
	if err != nil {
		return nil, err
	}
	result := make([]Metadata, 0, len(index))
	for _, item := range index {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (r Registry) Resolve(name string) (Definition, bool, error) {
	index, err := r.buildIndex()
	if err != nil {
		return Definition{}, false, err
	}
	meta, ok := index[strings.TrimSpace(name)]
	if !ok {
		return Definition{}, false, nil
	}
	definition, err := parseDefinition(meta.EntryPath, meta.Scope)
	if err != nil {
		return Definition{}, false, err
	}
	return definition, true, nil
}

func (r Registry) Match(instruction string) (Definition, bool, error) {
	index, err := r.buildIndex()
	if err != nil {
		return Definition{}, false, err
	}
	query := normalizeMatchText(instruction)
	bestScore := 0
	bestName := ""
	for name, meta := range index {
		score := matchScore(meta, query)
		if score > bestScore {
			bestScore = score
			bestName = name
		}
	}
	if bestName == "" {
		return Definition{}, false, nil
	}
	definition, err := parseDefinition(index[bestName].EntryPath, index[bestName].Scope)
	if err != nil {
		return Definition{}, false, err
	}
	return definition, true, nil
}

func (r Registry) buildIndex() (map[string]Metadata, error) {
	index := map[string]Metadata{}
	// User-level first, then project-level overrides.
	for i := len(r.sources) - 1; i >= 0; i-- {
		source := r.sources[i]
		entries, err := os.ReadDir(source.Root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			entryPath := filepath.Join(source.Root, entry.Name(), "SKILL.md")
			if _, err := os.Stat(entryPath); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			meta, err := parseMetadata(entryPath, source.Scope)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(meta.Name) == "" {
				meta.Name = entry.Name()
			}
			index[meta.Name] = meta
		}
	}
	return index, nil
}

func matchScore(meta Metadata, query string) int {
	if strings.TrimSpace(query) == "" {
		return 0
	}
	score := 0
	name := normalizeMatchText(meta.Name)
	description := normalizeMatchText(meta.Description)
	if name != "" && strings.Contains(query, name) {
		score += 10
	}
	for _, tag := range meta.Tags {
		tag = normalizeMatchText(tag)
		if tag != "" && strings.Contains(query, tag) {
			score += 6
		}
	}
	for _, token := range tokenize(description) {
		if len([]rune(token)) < 2 {
			continue
		}
		if strings.Contains(query, token) {
			score += 2
		}
	}
	return score
}

func normalizeMatchText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func tokenize(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",.;:!?()[]{}|/\\-_", r)
	})
}

func (m Metadata) ValidateAllowedTools(available map[string]struct{}) error {
	for _, toolName := range m.AllowedTools {
		if _, ok := available[toolName]; !ok {
			return fmt.Errorf("skill %s references unknown tool %s", m.Name, toolName)
		}
	}
	return nil
}
