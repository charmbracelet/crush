package terminal

type createOpts struct {
	args    []string
	env     map[string]string
	cwd     string
	byteLim *int
}

// CreateOption defines a single optional argument for Create.
type CreateOption func(*createOpts)

func WithArgs(a ...string) CreateOption {
	return func(o *createOpts) { o.args = a }
}

func WithEnv(e map[string]string) CreateOption {
	return func(o *createOpts) { o.env = e }
}

func WithCwd(dir string) CreateOption {
	return func(o *createOpts) { o.cwd = dir }
}

func WithByteLimit(n int) CreateOption {
	return func(o *createOpts) { o.byteLim = &n }
}
