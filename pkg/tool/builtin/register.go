package builtin

import (
	"giai/pkg/tool"
)

// RegisterAll registers all builtin tools to the provided registry.
// It uses RegisterInstance for stateless tools.
func RegisterAll(r *tool.Registry) {
	r.RegisterInstance(NewReadFile())
	r.RegisterInstance(NewBash())
	r.RegisterInstance(NewGlob())
	r.RegisterInstance(NewGrep())
}
