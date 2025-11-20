package prompt

import (
	"fmt"
	"strings"
)

// Template is a lightweight string template using double-brace placeholders.
// Example: "Hello {{name}}" with vars map{"name": "Agent"} -> "Hello Agent".
type Template struct {
	Text string
}

// NewTemplate returns a Template with the provided text.
func NewTemplate(text string) Template {
	return Template{Text: text}
}

// Render replaces all placeholders with values. Missing keys are left untouched.
func (t Template) Render(vars map[string]any) string {
	out := t.Text
	for key, val := range vars {
		out = strings.ReplaceAll(out, "{{"+key+"}}", fmt.Sprint(val))
	}
	return out
}
