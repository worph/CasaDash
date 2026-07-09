// Package composecmd invokes the `docker compose` plugin out-of-process. Using
// the plugin (rather than linking the compose engine) keeps the binary small and
// the behaviour identical to Docker's own tooling.
package composecmd

import (
	"context"
	"fmt"
	"os/exec"
)

// Up brings a project up in detached mode from the given compose files.
func Up(ctx context.Context, dir, project string, files, env []string) error {
	args := []string{"compose", "-p", project}
	for _, f := range files {
		args = append(args, "-f", f)
	}
	args = append(args, "up", "-d")

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compose up: %w: %s", err, out)
	}
	return nil
}
