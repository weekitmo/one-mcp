package market

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildUVVenvArgs(t *testing.T) {
	t.Run("adds clear when venv exists", func(t *testing.T) {
		tempDir := t.TempDir()
		venvDir := filepath.Join(tempDir, "venv")
		if err := os.MkdirAll(venvDir, 0o755); err != nil {
			t.Fatalf("failed to create venv dir: %v", err)
		}

		args, err := buildUVVenvArgs(venvDir)
		if err != nil {
			t.Fatalf("buildUVVenvArgs returned error: %v", err)
		}

		expected := []string{"venv", "--clear", venvDir}
		if len(args) != len(expected) {
			t.Fatalf("expected %d args, got %d (%v)", len(expected), len(args), args)
		}
		for i := range expected {
			if args[i] != expected[i] {
				t.Fatalf("expected args[%d]=%q, got %q", i, expected[i], args[i])
			}
		}
	})

	t.Run("does not add clear when venv missing", func(t *testing.T) {
		tempDir := t.TempDir()
		venvDir := filepath.Join(tempDir, "venv")

		args, err := buildUVVenvArgs(venvDir)
		if err != nil {
			t.Fatalf("buildUVVenvArgs returned error: %v", err)
		}

		expected := []string{"venv", venvDir}
		if len(args) != len(expected) {
			t.Fatalf("expected %d args, got %d (%v)", len(expected), len(args), args)
		}
		for i := range expected {
			if args[i] != expected[i] {
				t.Fatalf("expected args[%d]=%q, got %q", i, expected[i], args[i])
			}
		}
	})
}
