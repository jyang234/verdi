package experimentrun

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

func TestResumeRejectsNilServiceOrContext(t *testing.T) {
	var service *Service
	if _, err := service.Resume(context.Background(), ResumeRequest{}); err == nil || !strings.Contains(err.Error(), "service is nil") {
		t.Fatalf("nil service Resume error = %v", err)
	}
	if _, err := (&Service{}).Resume(nil, ResumeRequest{}); err == nil || !strings.Contains(err.Error(), "nil context") {
		t.Fatalf("nil context Resume error = %v", err)
	}
}

func TestResumeRefusesAbsentReceiptBeforeUsingExecutionPorts(t *testing.T) {
	root := t.TempDir()
	def, _, _ := testDefinition(t, []string{"alpha", "beta"}, 0)
	service := &Service{}
	_, err := service.Resume(context.Background(), ResumeRequest{
		Root: root, ExperimentDir: "experiments/comparison", Run: "run-1", Definition: def,
	})
	if err == nil || !strings.Contains(err.Error(), "requires an execution receipt") {
		t.Fatalf("Resume absent receipt error = %v", err)
	}
	paths, _ := experiment.PathsForRun("experiments/comparison", "run-1")
	for _, path := range []string{paths.Observations, paths.Result} {
		if _, statErr := os.Lstat(filepath.Join(root, filepath.FromSlash(path))); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("absent-receipt Resume created %s: %v", path, statErr)
		}
	}
}

func TestValidateResumeEnvironmentRootDistinguishesMissingAndActivatedState(t *testing.T) {
	root := t.TempDir()
	environment := filepath.Join(root, environmentRootName)
	if err := validateResumeEnvironmentRoot(root, environment, false); err != nil {
		t.Fatalf("missing root before measured evidence: %v", err)
	}
	if err := validateResumeEnvironmentRoot(root, environment, true); err == nil || !strings.Contains(err.Error(), "missing after measured evidence") {
		t.Fatalf("missing root after measured evidence error = %v", err)
	}

	if err := os.Mkdir(environment, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := validateResumeEnvironmentRoot(root, environment, false); err == nil || !strings.Contains(err.Error(), "activated profile shape") {
		t.Fatalf("empty collision error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(environment, ".home", ".config"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(environment, ".home", ".cache"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(environment, ".tmp"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := validateResumeEnvironmentRoot(root, environment, true); err != nil {
		t.Fatalf("activated environment root: %v", err)
	}
}

func TestValidateResumeEnvironmentRootRejectsCollisionKinds(t *testing.T) {
	for _, test := range []struct {
		name string
		make func(string) error
	}{
		{name: "regular file", make: func(path string) error { return os.WriteFile(path, []byte("collision"), 0o600) }},
		{name: "symlink", make: func(path string) error { return os.Symlink(t.TempDir(), path) }},
		{name: "partial activation", make: func(path string) error { return os.MkdirAll(filepath.Join(path, ".home"), 0o700) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, environmentRootName)
			if err := test.make(path); err != nil {
				t.Fatal(err)
			}
			if err := validateResumeEnvironmentRoot(root, path, false); err == nil {
				t.Fatal("validateResumeEnvironmentRoot collision = nil error")
			}
		})
	}
}

func TestCleanupEnvironmentRootsRemovesOnlyReservedRootsAndProvesAbsence(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "keep")
	if err := os.WriteFile(keep, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	var environments []string
	for _, candidate := range []string{"alpha", "beta"} {
		environment := filepath.Join(root, candidate, environmentRootName)
		if err := os.MkdirAll(filepath.Join(environment, ".home", ".config"), 0o700); err != nil {
			t.Fatal(err)
		}
		environments = append(environments, environment)
	}
	if err := cleanupEnvironmentRoots(root, environments); err != nil {
		t.Fatalf("cleanupEnvironmentRoots: %v", err)
	}
	for _, environment := range environments {
		if _, err := os.Lstat(environment); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("environment root %q remains: %v", environment, err)
		}
	}
	if got, err := os.ReadFile(keep); err != nil || string(got) != "keep" {
		t.Fatalf("cleanup changed non-reserved sibling: %q, %v", got, err)
	}
	if err := cleanupEnvironmentRoots(root, environments); err != nil {
		t.Fatalf("idempotent cleanup of absent roots: %v", err)
	}
}

func TestCleanupEnvironmentRootsRejectsNonDirectoryCollision(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, environmentRootName)
	if err := os.WriteFile(path, []byte("collision"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cleanupEnvironmentRoots(root, []string{path}); err == nil {
		t.Fatal("cleanupEnvironmentRoots(non-directory) = nil error")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "collision" {
		t.Fatalf("cleanup removed or changed collision: %q, %v", got, err)
	}
}

func TestCleanupEnvironmentRootsRefusesSymlinkedWorkspaceParent(t *testing.T) {
	root := t.TempDir()
	externalWorkspace := t.TempDir()
	externalEnvironment := filepath.Join(externalWorkspace, environmentRootName)
	if err := os.Mkdir(externalEnvironment, 0o700); err != nil {
		t.Fatal(err)
	}
	witness := filepath.Join(externalEnvironment, "must-remain")
	if err := os.WriteFile(witness, []byte("outside candidate workspace"), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(root, "candidate-workspace")
	if err := os.Symlink(externalWorkspace, workspace); err != nil {
		t.Fatal(err)
	}
	if err := cleanupEnvironmentRoots(root, []string{filepath.Join(workspace, environmentRootName)}); err == nil {
		t.Fatal("cleanupEnvironmentRoots(symlinked workspace parent) = nil error")
	}
	if got, err := os.ReadFile(witness); err != nil || string(got) != "outside candidate workspace" {
		t.Fatalf("cleanup traversed workspace symlink: %q, %v", got, err)
	}
}
