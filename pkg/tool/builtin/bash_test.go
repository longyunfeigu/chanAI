package builtin

import (
	"context"
	"strings"
	"testing"

	"giai/pkg/tool"
)

func TestBash_Execute(t *testing.T) {
	bash, err := NewBash(nil)
	if err != nil {
		t.Fatalf("NewBash() error = %v", err)
	}
	ctx := context.Background()
	tc := tool.NewToolContext()

	tests := []struct {
		name        string
		input       map[string]any
		wantOutput  string
		wantCode    int
		wantOK      bool
		wantErr     bool
	}{
		{
			name:       "Echo Command",
			input:      map[string]any{"cmd": "echo 'hello world'"},
			wantOutput: "hello world\n",
			wantCode:   0,
			wantOK:     true,
			wantErr:    false,
		},
		{
			name:       "Legacy Command Key",
			input:      map[string]any{"command": "echo 'legacy'"},
			wantOutput: "legacy\n",
			wantCode:   0,
			wantOK:     true,
			wantErr:    false,
		},
		{
			name:       "Exit Code 1",
			input:      map[string]any{"cmd": "exit 1"},
			wantOutput: "",
			wantCode:   1,
			wantOK:     false, // command returned non-zero exit
			wantErr:    false,
		},
		{
			name:       "Invalid Command",
			input:      map[string]any{"cmd": "invalid_command_xyz"},
			wantOutput: "",
			wantCode:   127, // Command not found (typical)
			wantOK:     false,
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

				okField, _ := res["ok"].(bool)
				if okField != tt.wantOK {
					t.Errorf("ok = %v, want %v", okField, tt.wantOK)
				}

				code, ok := res["code"].(int)
				if !ok {
					t.Fatalf("code not int, got %T", res["code"])
				}

				if code != tt.wantCode {
					// Note: Some shells might return slightly different codes for "not found".
					if tt.name == "Invalid Command" {
						combined, _ := res["output"].(string)
						if code != 127 && !strings.Contains(combined, "not found") {
							t.Errorf("code = %d, want %d", code, tt.wantCode)
						}
					} else {
						t.Errorf("code = %d, want %d", code, tt.wantCode)
					}
				}

				if tt.wantOutput != "" {
					output, _ := res["output"].(string)
					if output != tt.wantOutput {
						t.Errorf("output = %q, want %q", output, tt.wantOutput)
					}
				}
			}
		})
	}
}
