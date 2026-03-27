package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryProjectScopeOverridesUserScope(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	userRoot := t.TempDir()

	writeSkill(t, filepath.Join(userRoot, "weather-lookup", "SKILL.md"), `---
name: weather-lookup
description: user version
allowed-tools:
  - web.search
---
User body`)
	writeSkill(t, filepath.Join(projectRoot, "weather-lookup", "SKILL.md"), `---
name: weather-lookup
description: project version
allowed-tools:
  - web.search
  - web.fetch
tags:
  - 天气
---
Project body`)

	registry := NewRegistryWithSources([]Source{
		{Scope: ScopeProject, Root: projectRoot},
		{Scope: ScopeUser, Root: userRoot},
	})

	definition, ok, err := registry.Resolve("weather-lookup")
	if err != nil {
		t.Fatalf("resolve skill: %v", err)
	}
	if !ok {
		t.Fatal("expected skill to exist")
	}
	if definition.Description != "project version" {
		t.Fatalf("expected project skill to override user skill, got %#v", definition)
	}
	if len(definition.AllowedTools) != 2 {
		t.Fatalf("expected allowed tools to parse, got %#v", definition.AllowedTools)
	}
}

func TestRegistryMatchesWeatherLookupByDescriptionAndTags(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	writeSkill(t, filepath.Join(projectRoot, "weather-lookup", "SKILL.md"), `---
name: weather-lookup
description: 查询城市实时天气并给出来源
allowed-tools:
  - web.search
  - web.fetch
tags:
  - 天气
  - 温度
---
Weather instructions`)

	registry := NewRegistryWithSources([]Source{
		{Scope: ScopeProject, Root: projectRoot},
	})

	definition, ok, err := registry.Match("武汉天气怎么样")
	if err != nil {
		t.Fatalf("match skill: %v", err)
	}
	if !ok {
		t.Fatal("expected weather skill to match")
	}
	if definition.Name != "weather-lookup" {
		t.Fatalf("unexpected match: %#v", definition)
	}
	if definition.Instructions != "Weather instructions" {
		t.Fatalf("expected instructions to load on match, got %#v", definition)
	}
}

func writeSkill(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}
