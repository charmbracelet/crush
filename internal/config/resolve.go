package config

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/shell"
)

type VariableResolver interface {
	ResolveValue(value string) (string, error)
}

type Shell interface {
	Exec(ctx context.Context, command string) (stdout, stderr string, err error)
}

type shellVariableResolver struct {
	shell Shell
	env   env.Env
}

func NewShellVariableResolver(env env.Env) VariableResolver {
	return &shellVariableResolver{
		env: env,
		shell: shell.NewShell(
			&shell.Options{
				Env: env.Env(),
			},
		),
	}
}

// ResolveValue is a method for resolving values, such as environment variables.
// it will expect strings that start with `$` to be resolved as environment variables or shell commands.
// if the string does not start with `$`, it will return the string as is.
func (r *shellVariableResolver) ResolveValue(value string) (string, error) {
	if !strings.HasPrefix(value, "$") {
		return value, nil
	}

	if strings.HasPrefix(value, "$(") && strings.HasSuffix(value, ")") {
		command := strings.TrimSuffix(strings.TrimPrefix(value, "$("), ")")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		stdout, _, err := r.shell.Exec(ctx, command)
		if err != nil {
			return "", fmt.Errorf("command execution failed: %w", err)
		}
		return strings.TrimSpace(stdout), nil
	}

	if after, ok := strings.CutPrefix(value, "$"); ok {
		varName := after
		value = r.env.Get(varName)
		if value == "" {
			return "", fmt.Errorf("environment variable %q not set", varName)
		}
		return value, nil
	}
	return "", fmt.Errorf("invalid value format: %s", value)
}

type environmentVariableResolver struct {
	env env.Env
}

func NewEnvironmentVariableResolver(env env.Env) VariableResolver {
	return &environmentVariableResolver{
		env: env,
	}
}

// ResolveValue resolves environment variables from the provided env.Env.
func (r *environmentVariableResolver) ResolveValue(value string) (string, error) {
	if !strings.HasPrefix(value, "$") {
		return value, nil
	}

	varName := strings.TrimPrefix(value, "$")
	resolvedValue := r.env.Get(varName)
	if resolvedValue == "" {
		return "", fmt.Errorf("environment variable %q not set", varName)
	}
	return resolvedValue, nil
}
