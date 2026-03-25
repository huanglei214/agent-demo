package app

type ToolDescriptor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s Services) ListTools() []ToolDescriptor {
	return s.toolDescriptors()
}
