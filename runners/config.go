package runners

type Config struct {
	ServerUrl string
	Workspace string
	Limit     int
	Plugin    []string
	Env       []string
}
