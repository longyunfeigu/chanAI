package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"giai/pkg/tool"
)

func TestReadFile_Execute(t *testing.T) {
	// Create a temporary file for testing
	tmpContent := "line1\nline2\nline3\nline4"
	tmpFile, err := os.CreateTemp("", "giai_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name()) // Clean up

	if _, err := tmpFile.WriteString(tmpContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()
	absPath, _ := filepath.Abs(tmpFile.Name())

	rf, err := NewReadFile(nil)
	if err != nil {
		t.Fatalf("NewReadFile() error = %v", err)
	}
	ctx := context.Background()
	tc := tool.NewToolContext()

	tests := []struct {
		name        string
		input       map[string]any
		wantContent string
		wantOK      bool
		wantErr     bool
	}{
		{
			name:        "Valid Read Full",
			input:       map[string]any{"path": absPath},
			wantContent: tmpContent,
			wantOK:      true,
			wantErr:     false,
		},
		{
			name: "Offset And Limit",
			input: map[string]any{
				"path":   absPath,
				"offset": 1,
				"limit":  2,
			},
			wantContent: "line2\nline3",
			wantOK:      true,
			wantErr:     false,
		},
		{
			name:        "Relative Path Not OK",
			input:       map[string]any{"path": "relative/path.txt"},
			wantContent: "",
			wantOK:      false,
			wantErr:     false,
		},
		{
			name:        "Non-existent File Not OK",
			input:       map[string]any{"path": filepath.Join(os.TempDir(), "non_existent_file_123.txt")},
			wantContent: "",
			wantOK:      false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rf.Execute(ctx, tt.input, tc)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// When no Go error is returned, we always expect a structured map result.
			res, ok := got.(map[string]any)
			if !ok {
				t.Fatalf("Execute() expected map[string]any result, got %T", got)
			}

			okField, _ := res["ok"].(bool)
			if okField != tt.wantOK {
				t.Errorf("ok = %v, want %v", okField, tt.wantOK)
			}

			if tt.wantContent != "" {
				content, _ := res["content"].(string)
				if content != tt.wantContent {
					t.Errorf("content = %q, want %q", content, tt.wantContent)
				}
			}
		})
	}
}
