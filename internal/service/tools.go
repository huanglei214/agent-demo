package service

import "github.com/huanglei214/agent-demo/internal/tool"

type ToolDescriptor struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Access      tool.AccessMode `json:"access"`
}

func (s Services) ListTools() []ToolDescriptor {
	descriptors := s.ToolRegistry.Descriptors()
	result := make([]ToolDescriptor, 0, len(descriptors))
	for _, item := range descriptors {
		result = append(result, ToolDescriptor{
			Name:        item.Name,
			Description: item.Description,
			Access:      item.Access,
		})
	}
	return result
}
