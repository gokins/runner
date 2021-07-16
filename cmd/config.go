package cmd

type Config struct {
	Name   string   `yaml:"name"`
	Host   string   `yaml:"host"`
	Secret string   `yaml:"secret"`
	Limit  int      `yaml:"limit"`
	Plugin []string `yaml:"plugin"`
	Env    []string `yaml:"env"`

	WorkPath string `yaml:"workPath"`
}
