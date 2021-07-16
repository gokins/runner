package runners

type Config struct {
	Name      string
	Workspace string
	Limit     int
	Plugin    []string
	Env       []string
}
