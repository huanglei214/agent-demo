package app

import "github.com/huanglei214/agent-demo/internal/tool"

type ToolDescriptor struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Access      tool.AccessMode `json:"access"`
}

func (s Services) ListTools() []ToolDescriptor {
	return s.toolDescriptors()
}
