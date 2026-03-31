package prompt

import (
	"strings"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func InjectTodoContext(base Prompt, run harnessruntime.Run, state harnessruntime.RunState) Prompt {
	if run.PlanMode != harnessruntime.PlanModeTodo {
		return base
	}

	if base.Metadata == nil {
		base.Metadata = map[string]any{}
	}
	base.Metadata["plan_mode"] = string(run.PlanMode)
	base.Metadata["todo_count"] = len(state.Todos)
	if len(state.Todos) == 0 {
		return base
	}

	section := strings.Join([]string{
		"Todo snapshot:\n" + harnessruntime.MustJSON(state.Todos),
		"Todo rules:\n- Use the `todo` action for complex tasks when helpful.\n- When updating todos, send the complete latest list with operation `set`.\n- Use `set` with an empty list to clear todos.\n- Do not overuse todos for trivial tasks.",
	}, "\n\n")

	base.Input = strings.TrimSpace(strings.Join([]string{base.Input, section}, "\n\n"))
	return base
}
