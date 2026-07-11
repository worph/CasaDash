package composecmd

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Config resolves a project the way `docker compose up` would — merging the
// files, interpolating the variables, applying the override tags — and returns
// the result. It is what the settings window shows as the "effective" compose:
// exactly what the two files add up to, which is otherwise invisible.
func Config(ctx context.Context, dir, project string, files, env []string) (string, error) {
	out, err := config(ctx, dir, project, files, env, false)
	if err != nil {
		return "", err
	}
	return out, nil
}

// Validate parses a project without resolving it fully, returning the error
// Compose itself reports. It is the guard in front of every override save: a
// typo'd YAML or an unknown key is caught here rather than by watching the stack
// fail to come back up.
func Validate(ctx context.Context, dir, project string, files, env []string) error {
	_, err := config(ctx, dir, project, files, env, true)
	return err
}

func config(ctx context.Context, dir, project string, files, env []string, quiet bool) (string, error) {
	args := []string{"compose", "-p", project}
	for _, f := range files {
		args = append(args, "-f", f)
	}
	args = append(args, "config")
	if quiet {
		args = append(args, "--quiet")
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	cmd.Env = env

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s", composeError(err))
	}
	return string(out), nil
}

// composeError surfaces what compose printed on stderr — "services.app.ports.0:
// invalid port" is the whole point of validating, and it is far more useful than
// "exit status 15".
func composeError(err error) string {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if msg := strings.TrimSpace(string(ee.Stderr)); msg != "" {
			return msg
		}
	}
	return err.Error()
}
