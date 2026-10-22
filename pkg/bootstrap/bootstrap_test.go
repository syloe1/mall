package bootstrap

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// ─── SetGinMode ───────────────────────────────────────────────────────────────

func TestSetGinMode(t *testing.T) {
	t.Run("release", func(t *testing.T) {
		SetGinMode("release")
		assert.Equal(t, gin.ReleaseMode, gin.Mode())
		gin.SetMode(gin.TestMode) // restore
	})

	t.Run("prod", func(t *testing.T) {
		SetGinMode("prod")
		assert.Equal(t, gin.ReleaseMode, gin.Mode())
		gin.SetMode(gin.TestMode)
	})

	t.Run("test", func(t *testing.T) {
		SetGinMode("test")
		assert.Equal(t, gin.TestMode, gin.Mode())
	})

	t.Run("default_empty_string", func(t *testing.T) {
		SetGinMode("")
		assert.Equal(t, gin.DebugMode, gin.Mode())
		gin.SetMode(gin.TestMode)
	})

	t.Run("unknown_value", func(t *testing.T) {
		SetGinMode("unknown-mode")
		assert.Equal(t, gin.DebugMode, gin.Mode())
		gin.SetMode(gin.TestMode)
	})
}

// ─── MustClose ────────────────────────────────────────────────────────────────

func TestMustClose(t *testing.T) {
	t.Run("no_error_does_not_panic", func(t *testing.T) {
		called := false
		MustClose("resource", func() error {
			called = true
			return nil
		})
		assert.True(t, called)
	})

	t.Run("error_does_not_panic", func(t *testing.T) {
		called := false
		MustClose("resource", func() error {
			called = true
			return errors.New("close failed")
		})
		assert.True(t, called)
	})
}

// ─── ResolveConfigPathFromCandidates ──────────────────────────────────────────

func TestResolveConfigPathFromCandidates(t *testing.T) {
	t.Run("first_candidate_exists", func(t *testing.T) {
		dir := t.TempDir()
		cfgFile := filepath.Join(dir, "config.yaml")
		assert.NoError(t, os.WriteFile(cfgFile, []byte("key: value"), 0o644))

		path, err := ResolveConfigPathFromCandidates([]string{
			cfgFile,
			"/nonexistent/path.yaml",
		})
		assert.NoError(t, err)
		assert.Equal(t, cfgFile, path)
	})

	t.Run("second_candidate_exists", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "missing.yaml")
		existing := filepath.Join(dir, "found.yaml")
		assert.NoError(t, os.WriteFile(existing, []byte("key: value"), 0o644))

		path, err := ResolveConfigPathFromCandidates([]string{missing, existing})
		assert.NoError(t, err)
		assert.Equal(t, existing, path)
	})

	t.Run("none_exists", func(t *testing.T) {
		_, err := ResolveConfigPathFromCandidates([]string{
			"/tmp/not-exist-001.yaml",
			"/tmp/not-exist-002.yaml",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config file not found")
	})

	t.Run("empty_candidates", func(t *testing.T) {
		_, err := ResolveConfigPathFromCandidates([]string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config file not found")
	})

	t.Run("parametrized", func(t *testing.T) {
		dir := t.TempDir()
		create := func(name string) string {
			f := filepath.Join(dir, name)
			os.WriteFile(f, []byte("ok"), 0o644)
			return f
		}
		a := create("a.yaml")
		b := create("b.yaml")
		c := create("c.yaml")

		type testCase struct {
			name     string
			cands    []string
			wantPath string
			wantErr  bool
		}
		tests := []testCase{
			{name: "first", cands: []string{a, b, c}, wantPath: a, wantErr: false},
			{name: "second", cands: []string{"/nonexistent", b, c}, wantPath: b, wantErr: false},
			{name: "last", cands: []string{"/nonexistent_1", "/nonexistent_2", c}, wantPath: c, wantErr: false},
			{name: "none", cands: []string{"/nx1", "/nx2", "/nx3"}, wantPath: "", wantErr: true},
			{name: "empty", cands: []string{}, wantPath: "", wantErr: true},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				path, err := ResolveConfigPathFromCandidates(tc.cands)
				if tc.wantErr {
					assert.Error(t, err)
					return
				}
				assert.NoError(t, err)
				assert.Equal(t, tc.wantPath, path)
			})
		}
	})
}
