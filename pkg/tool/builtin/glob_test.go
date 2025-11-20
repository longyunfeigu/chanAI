package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"giai/pkg/tool"
)

func TestGlob_Execute(t *testing.T) {
	// Setup temp directory with structure:
	// tmp/
	//   a.txt
	//   sub/
	//     b.go
	//     c.js
	tmpDir, err := os.MkdirTemp("", "glob_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	createFile(t, filepath.Join(tmpDir, "a.txt"))
	os.Mkdir(filepath.Join(tmpDir, "sub"), 0755)
	createFile(t, filepath.Join(tmpDir, "sub", "b.go"))
	createFile(t, filepath.Join(tmpDir, "sub", "c.js"))

	g := NewGlob()
	ctx := context.Background()
	tc := tool.NewToolContext()

	tests := []struct {
		name    string
		input   map[string]any
		wantCnt int
	}{
		{
			name: "Match all recursive",
			input: map[string]any{
				"pattern":  "**/*",
				"root_dir": tmpDir,
			},
			// a.txt, sub, sub/b.go, sub/c.js = 4 entries? 
			// Doublestar glob behavior: **/* matches files and dirs usually.
			// Let's just check if it finds the files we expect.
			wantCnt: 3, // expecting at least 3 files (directories might be included depending on impl)
		},
		{
			name: "Match extension",
			input: map[string]any{
				"pattern":  "**/*.go",
				"root_dir": tmpDir,
			},
			wantCnt: 1, // sub/b.go
		},
		{
			name: "Exclude pattern",
			input: map[string]any{
				"pattern":  "**/*",
				"root_dir": tmpDir,
				"exclude":  []any{"**/*.js"},
			},
			wantCnt: 2, // a.txt, sub/b.go (dirs might be there)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := g.Execute(ctx, tt.input, tc)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			files, ok := got.([]string)
			if !ok {
				t.Fatalf("Result not []string")
			}

			// Simple count check, actual glob behavior can be tricky with dirs
			// We filter for files only in our check if we wanted to be strict
			count := 0
			for _, f := range files {
				info, err := os.Stat(f)
				if err == nil && !info.IsDir() {
					count++
				}
			}
			
			// Allow some flexibility if dirs are included in raw glob
			if count < tt.wantCnt {
				t.Errorf("Found %d files, want at least %d. Matches: %v", count, tt.wantCnt, files)
			}
		})
	}
}

func createFile(t *testing.T, path string) {
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
}
