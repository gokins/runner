package cmd

type Config struct {
	Name   string `yaml:"name"`
	Host   string `yaml:"host"`
	Secret string `yaml:"secret"`
}
