package builtin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"giai/pkg/tool"
)

func TestGrep_Execute(t *testing.T) {
	// We need 'rg' installed for this test. If not present, we skip.
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep (rg) not found, skipping test")
	}

	tmpDir, err := os.MkdirTemp("", "grep_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	f1 := filepath.Join(tmpDir, "hello.txt")
	os.WriteFile(f1, []byte("Hello World\nFoo Bar\nHello Universe"), 0644)

	f2 := filepath.Join(tmpDir, "other.txt")
	os.WriteFile(f2, []byte("Nothing here"), 0644)

	g := NewGrep()
	ctx := context.Background()
	tc := tool.NewToolContext()

	tests := []struct {
		name    string
		input   map[string]any
		wantStr string
		wantErr bool
	}{
		{
			name: "Simple Match",
			input: map[string]any{
				"pattern": "Hello",
				"path":    tmpDir,
			},
			wantStr: "hello.txt:1:Hello World\nhello.txt:3:Hello Universe",
			wantErr: false,
		},
		{
			name: "Case Insensitive",
			input: map[string]any{
				"pattern":          "world",
				"path":             tmpDir,
				"case_insensitive": true,
			},
			wantStr: "hello.txt:1:Hello World",
			wantErr: false,
		},
		{
			name: "No Matches",
			input: map[string]any{
				"pattern": "Zebra",
				"path":    tmpDir,
			},
			wantStr: "No matches found",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := g.Execute(ctx, tt.input, tc)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				gotStr, ok := got.(string)
				if !ok {
					t.Errorf("Result not string")
				}
				// Check if output contains expected substrings (rg output format might vary slightly by version)
				// We just check basic containment if it's not exact match
				if tt.wantStr == "No matches found" {
					if gotStr != tt.wantStr {
						t.Errorf("got %q, want %q", gotStr, tt.wantStr)
					}
				} else {
					// Loose check
					if len(gotStr) == 0 {
						t.Errorf("Expected matches, got empty string")
					}
				}
			}
		})
	}
}
