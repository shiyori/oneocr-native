package engine

import (
	"path/filepath"
	"testing"
)

// Fixture-based integration tests keep the repository-relative paths shared
// with reference scripts. t.Chdir restores the package directory after each test.
func useRepositoryFixtures(t *testing.T) {
	t.Helper()
	t.Chdir(filepath.Join("..", ".."))
}
