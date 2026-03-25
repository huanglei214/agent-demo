package tool

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *Registry) List() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

func (r *Registry) Descriptors() []Descriptor {
	tools := r.List()
	result := make([]Descriptor, 0, len(tools))
	for _, item := range tools {
		result = append(result, Descriptor{
			Name:        item.Name(),
			Description: item.Description(),
			Access:      item.AccessMode(),
		})
	}
	return result
}
