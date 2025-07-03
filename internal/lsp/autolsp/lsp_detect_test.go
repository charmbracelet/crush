package autolsp_test

import (
	"os"
	"testing"

	"github.com/charmbracelet/crush/internal/lsp/autolsp"
)

func TestServerDetectorInstalled(t *testing.T) {
	lookPathFunc := func(name string) (string, error) {
		if name == "gopls" {
			return "/usr/local/bin/gopls", nil
		}
		return "", os.ErrNotExist
	}

	d := autolsp.NewServerDetector(
		autolsp.ServerDetectorWithLangs(autolsp.LangGo),
		autolsp.ServerDetectorWithLookPathFunc(lookPathFunc),
	)
	installed, toBeInstalled := d.Detect()

	if len(installed) != 1 || installed[0].Name != autolsp.ServerGopls {
		t.Errorf("expected gopls to be installed, got %v", installed)
	}
	if len(toBeInstalled) != 0 {
		t.Errorf("expected no servers to be installed, got %v", toBeInstalled)
	}
}

func TestServerDetectorNotInstalled(t *testing.T) {
	lookPathFunc := func(name string) (string, error) {
		return "", os.ErrNotExist
	}

	d := autolsp.NewServerDetector(
		autolsp.ServerDetectorWithLangs(autolsp.LangGo),
		autolsp.ServerDetectorWithLookPathFunc(lookPathFunc),
	)
	installed, toBeInstalled := d.Detect()

	if len(installed) != 0 {
		t.Errorf("expected no servers to be installed, got %v", installed)
	}
	if len(toBeInstalled) != 1 || toBeInstalled[0].Name != autolsp.ServerGopls {
		t.Errorf("expected gopls to be in the list of servers to be installed, got %v", toBeInstalled)
	}
}

func TestServerDetectorPriorityOrderInstalled(t *testing.T) {
	// Mock function that returns all Python servers as installed
	lookPathFunc := func(name string) (string, error) {
		if name == "jedi-language-server" || name == "pylsp" || name == "pyright" {
			return "/usr/local/bin/" + name, nil
		}
		return "", os.ErrNotExist
	}

	// Test with python first - should prioritize python servers in order they appear in Servers slice
	d := autolsp.NewServerDetector(
		autolsp.ServerDetectorWithLangs(autolsp.LangPython, autolsp.LangGo),
		autolsp.ServerDetectorWithLookPathFunc(lookPathFunc),
	)
	installed, _ := d.Detect()

	if len(installed) != 3 {
		t.Errorf("expected 3 python servers to be installed, got %d", len(installed))
	}

	// Verify they are in the priority order (jedi-language-server, pylsp, pyright)
	expectedOrder := []string{"jedi-language-server", "pylsp", "pyright"}
	for i, server := range installed {
		if string(server.Name) != expectedOrder[i] {
			t.Errorf("expected server at index %d to be %s, got %s", i, expectedOrder[i], server.Name)
		}
	}
}

func TestServerDetectorPriorityOrderToBeInstalled(t *testing.T) {
	// Mock function that returns no servers as installed
	lookPathFunc := func(name string) (string, error) {
		return "", os.ErrNotExist
	}

	// Test with multiple languages - go should come first, then python servers
	d := autolsp.NewServerDetector(
		autolsp.ServerDetectorWithLangs(autolsp.LangGo, autolsp.LangPython),
		autolsp.ServerDetectorWithLookPathFunc(lookPathFunc),
	)
	_, toBeInstalled := d.Detect()

	if len(toBeInstalled) != 4 {
		t.Errorf("expected 4 servers to be installed (1 go + 3 python), got %d", len(toBeInstalled))
	}

	// First server should be gopls (go language comes first in langs slice)
	if toBeInstalled[0].Name != autolsp.ServerGopls {
		t.Errorf("expected first server to be gopls, got %s", toBeInstalled[0].Name)
	}

	// Remaining servers should be python servers in their priority order
	expectedPythonOrder := []string{"jedi-language-server", "pylsp", "pyright"}
	for i, expectedName := range expectedPythonOrder {
		if string(toBeInstalled[i+1].Name) != expectedName {
			t.Errorf("expected python server at index %d to be %s, got %s", i+1, expectedName, toBeInstalled[i+1].Name)
		}
	}
}
