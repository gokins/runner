package runners

type Config struct {
	Workspace string
	Limit     int
	Plugin    []string
	Env       []string
}
