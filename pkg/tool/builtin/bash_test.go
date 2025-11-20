package builtin

import (
	"context"
	"strings"
	"testing"

	"giai/pkg/tool"
)

func TestBash_Execute(t *testing.T) {
	bash := NewBash()
	ctx := context.Background()
	tc := tool.NewToolContext()

	tests := []struct {
		name       string
		input      map[string]any
		wantStdOut string
		wantCode   int
		wantErr    bool
	}{
		{
			name:       "Echo Command",
			input:      map[string]any{"command": "echo 'hello world'"},
			wantStdOut: "hello world\n",
			wantCode:   0,
			wantErr:    false,
		},
		{
			name:       "Exit Code 1",
			input:      map[string]any{"command": "exit 1"},
			wantStdOut: "",
			wantCode:   1,
			wantErr:    false, // Tool execution itself succeeded, but command returned 1
		},
		{
			name:       "Invalid Command",
			input:      map[string]any{"command": "invalid_command_xyz"},
			wantStdOut: "",
			wantCode:   127, // Command not found
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bash.Execute(ctx, tt.input, tc)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr {
				res, ok := got.(map[string]any)
				if !ok {
					t.Fatalf("Result not a map")
				}
				
				stdout := res["stdout"].(string)
				code := res["code"].(int)

				if stdout != tt.wantStdOut {
					t.Errorf("stdout = %q, want %q", stdout, tt.wantStdOut)
				}
				if code != tt.wantCode {
					// Note: Some shells might return slightly different codes for not found, 
					// but 127 is standard for bash.
					if tt.name == "Invalid Command" && code != 127 && !strings.Contains(res["stderr"].(string), "not found") {
						t.Errorf("code = %d, want %d", code, tt.wantCode)
					} else if tt.name != "Invalid Command" {
						t.Errorf("code = %d, want %d", code, tt.wantCode)
					}
				}
			}
		})
	}
}
