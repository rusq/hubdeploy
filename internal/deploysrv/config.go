package deploysrv

import (
	"errors"
	"io"
	"os"

	"github.com/goccy/go-yaml"

	"github.com/rusq/dlog"
)

type Config struct {
	FQDN        string       `yaml:"url"` // server url for results callback
	Cert        string       `yaml:"cert"`
	Key         string       `yaml:"key"`
	ResultsDir  string       `yaml:"results_dir"`
	Deployments []Deployment `yaml:"deployments"`
}

type Deployment struct {
	RepoName string   `yaml:"repo_name"`
	Tags     []string `yaml:"tags"`
	Disabled bool     `yaml:"disabled"`
	Workdir  string   `yaml:"work_dir"`
	Command  []string `yaml:"command"`
}

func (c *Config) IsEmpty() bool {
	n := 0
	for _, d := range c.Deployments {
		if !d.Disabled {
			n++
		}
	}
	return n == 0
}

func (c *Config) repomap() map[string]Deployment {
	m := make(map[string]Deployment)
	for _, repo := range c.Deployments {
		m[repo.RepoName] = repo
	}
	return m
}

func (c *Config) validate() error {
	for i := range c.Deployments {
		if err := c.Deployments[i].check(); err != nil {
			return err
		}
	}
	if c.IsEmpty() {
		return errors.New("all configurations are invalid or empty config")
	}
	return nil
}

func (m *Deployment) check() error {
	if m.RepoName == "" {
		return errors.New("empty repository name")
	}
	if m.Disabled {
		return nil
	}
	fi, err := os.Stat(m.Workdir)
	if err != nil {
		m.Disabled = true
		dlog.Printf("[%s] %s", m.RepoName, err)
		return nil
	}
	if !fi.IsDir() {
		m.Disabled = true
		dlog.Printf("[%s] %s is not a directory", m.RepoName, m.Workdir)
		return nil
	}
	return nil
}

func LoadConfig(filename string) (Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	return readConfig(f)
}

func readConfig(r io.Reader) (Config, error) {
	dec := yaml.NewDecoder(r, yaml.DisallowUnknownField(), yaml.DisallowDuplicateKey())
	var c Config
	if err := dec.Decode(&c); err != nil {
		return Config{}, err
	}
	return c, nil
}
