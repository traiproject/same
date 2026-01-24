//go:build e2e

package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

var sameBinary string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "same-e2e-*")
	if err != nil {
		panic(err)
	}

	sameBinary = filepath.Join(tmpDir, "same")

	//nolint:gosec // Building binary with static arguments, not user input
	cmd := exec.Command("nix", "develop", "-c", "go", "build", "-o", sameBinary, "./cmd/same")
	cmd.Dir = filepath.Join("..", "..")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		panic("failed to build same binary: " + err.Error())
	}

	exitCode := m.Run()

	_ = os.RemoveAll(tmpDir)

	os.Exit(exitCode)
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:   "testdata",
		Setup: setupE2E,
	})
}

func setupE2E(env *testscript.Env) error {
	env.Setenv("NO_COLOR", "1")
	env.Setenv("CI", "true")

	binDir := filepath.Dir(sameBinary)
	currentPath := env.Getenv("PATH")
	env.Setenv("PATH", binDir+string(os.PathListSeparator)+currentPath)

	homeDir := filepath.Join(env.WorkDir, ".home")
	if err := os.MkdirAll(homeDir, 0o750); err != nil {
		return err
	}
	env.Setenv("HOME", homeDir)

	return nil
}
