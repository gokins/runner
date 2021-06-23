package runners

type Config struct {
	ServAddr   string
	ServSecret string
	Name       string
	Workspace  string
	Limit      int
	Plugin     []string
}
