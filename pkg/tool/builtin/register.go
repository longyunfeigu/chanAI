package builtin

import "giai/pkg/tool"

// RegisterAll registers all builtin tools into the registry.
// We follow the same设计思路 as .tmp_agentsdk/pkg/tools/builtin/registry.go:
// - Register tools by name
// - Use factories so tools can later accept config if needed
func RegisterAll(r *tool.Registry) {
	// File-system / code navigation related tools
	r.RegisterFactory("read_file", NewReadFile)
	r.RegisterFactory("glob", NewGlob)
	r.RegisterFactory("grep", NewGrep)

	// Shell execution
	r.RegisterFactory("bash", NewBash)
}

// FileSystemTools returns the builtin tools that primarily work with the local filesystem.
// Aligns conceptually with agentsdk's FileSystemTools helper.
func FileSystemTools() []string {
	return []string{"read_file", "glob", "grep"}
}

// BashTools returns tools that execute shell commands.
func BashTools() []string {
	return []string{"bash"}
}

// AllTools returns all builtin tool names.
func AllTools() []string {
	var tools []string
	tools = append(tools, FileSystemTools()...)
	tools = append(tools, BashTools()...)
	return tools
}
