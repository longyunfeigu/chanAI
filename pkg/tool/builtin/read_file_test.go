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
	tmpContent := "Hello, Giai!"
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

	rf := NewReadFile()
	ctx := context.Background()
	tc := tool.NewToolContext()

	tests := []struct {
		name    string
		input   map[string]any
		want    string
		wantErr bool
	}{
		{
			name:    "Valid Read",
			input:   map[string]any{"path": absPath},
			want:    tmpContent,
			wantErr: false,
		},
		{
			name:    "Relative Path Error",
			input:   map[string]any{"path": "relative/path.txt"},
			want:    "",
			wantErr: true,
		},
		{
			name:    "Non-existent File",
			input:   map[string]any{"path": filepath.Join(os.TempDir(), "non_existent_file_123.txt")},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rf.Execute(ctx, tt.input, tc)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				gotStr, ok := got.(string)
				if !ok {
					t.Errorf("Execute() returned non-string result")
				}
				if gotStr != tt.want {
					t.Errorf("Execute() = %v, want %v", gotStr, tt.want)
				}
			}
		})
	}
}
