package prompt

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

type templates struct {
	base                    string
	leadRole                string
	subagentRole            string
	leadTaskGuidance        string
	subagentTaskGuidance    string
	leadFollowUpRule        string
	subagentFollowUpRule    string
	leadForcedFinalRule     string
	subagentForcedFinalRule string
}

//go:embed templates/*.txt
var embeddedTemplates embed.FS

func defaultTemplates() templates {
	loaded, err := loadTemplates(embeddedTemplates)
	if err != nil {
		panic(err)
	}
	return loaded
}

func loadTemplates(fsys fs.FS) (templates, error) {
	files := map[string]*string{
		"templates/base.txt":                       nil,
		"templates/lead_role.txt":                  nil,
		"templates/subagent_role.txt":              nil,
		"templates/lead_task_guidance.txt":         nil,
		"templates/subagent_task_guidance.txt":     nil,
		"templates/lead_follow_up_rule.txt":        nil,
		"templates/subagent_follow_up_rule.txt":    nil,
		"templates/lead_forced_final_rule.txt":     nil,
		"templates/subagent_forced_final_rule.txt": nil,
	}

	result := templates{}
	files["templates/base.txt"] = &result.base
	files["templates/lead_role.txt"] = &result.leadRole
	files["templates/subagent_role.txt"] = &result.subagentRole
	files["templates/lead_task_guidance.txt"] = &result.leadTaskGuidance
	files["templates/subagent_task_guidance.txt"] = &result.subagentTaskGuidance
	files["templates/lead_follow_up_rule.txt"] = &result.leadFollowUpRule
	files["templates/subagent_follow_up_rule.txt"] = &result.subagentFollowUpRule
	files["templates/lead_forced_final_rule.txt"] = &result.leadForcedFinalRule
	files["templates/subagent_forced_final_rule.txt"] = &result.subagentForcedFinalRule

	for path, target := range files {
		content, err := fs.ReadFile(fsys, path)
		if err != nil {
			return templates{}, fmt.Errorf("load prompt template %s: %w", path, err)
		}
		*target = strings.TrimSpace(string(content))
	}

	return result, nil
}
