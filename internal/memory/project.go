package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os/exec"
	"path/filepath"
	"strings"
)

func ResolveProject(ctx context.Context, workingDir string) Project {
	root := canonicalPath(workingDir)
	identityPath := root

	if gitRoot, ok := gitPath(ctx, workingDir, "--show-toplevel"); ok {
		root = canonicalPath(gitRoot)
		identityPath = root
		if commonDir, commonOK := gitPath(ctx, root, "--git-common-dir"); commonOK {
			if !filepath.IsAbs(commonDir) {
				commonDir = filepath.Join(root, commonDir)
			}
			identityPath = canonicalPath(commonDir)
		}
	}

	key := strings.ToLower(filepath.ToSlash(identityPath))
	sum := sha256.Sum256([]byte(key))
	return Project{
		ID:   hex.EncodeToString(sum[:12]),
		Name: filepath.Base(root),
		Root: root,
	}
}

func gitPath(ctx context.Context, dir string, arg string) (string, bool) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", arg)
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(string(out))
	return value, value != ""
}

func canonicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	if evaluated, evalErr := filepath.EvalSymlinks(path); evalErr == nil {
		path = evaluated
	}
	return filepath.Clean(path)
}
