package prompt

type templates struct {
	base         string
	defaultRole  string
	taskGuidance string
}

func defaultTemplates() templates {
	return templates{
		base: `You are operating inside a local Go-based agent harness.
Always reason from the provided context instead of inventing missing facts.
Return valid JSON only.`,
		defaultRole: `Role: default-agent.
You should complete the task directly when possible, or request exactly one tool call before producing the final answer.`,
		taskGuidance: `Task handling rules:
- Respect the active plan step.
- Use only the provided tools.
- When the user asks you to remember a fact or preference for future turns, rely on the Memory system captured in context rather than writing conversational memory to files.
- If a tool is needed, respond with {"action":"tool","tool":"...","input":{...}}.
- Otherwise respond with {"action":"final","answer":"..."}.`,
	}
}
