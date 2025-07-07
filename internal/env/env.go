package env

import "os"

type Env interface {
	Get(key string) string
	Env() []string
}

type osEnv struct{}

// Get implements Env.
func (o *osEnv) Get(key string) string {
	return os.Getenv(key)
}

func (o *osEnv) Env() []string {
	env := os.Environ()
	if len(env) == 0 {
		return nil
	}
	return env
}

func New() Env {
	return &osEnv{}
}

type mapEnv struct {
	m map[string]string
}

// Get implements Env.
func (m *mapEnv) Get(key string) string {
	if value, ok := m.m[key]; ok {
		return value
	}
	return ""
}

// Env implements Env.
func (m *mapEnv) Env() []string {
	if len(m.m) == 0 {
		return nil
	}
	env := make([]string, 0, len(m.m))
	for k, v := range m.m {
		env = append(env, k+"="+v)
	}
	return env
}

func NewFromMap(m map[string]string) Env {
	if m == nil {
		m = make(map[string]string)
	}
	return &mapEnv{m: m}
}
