package deploysrv

import (
	"errors"
	"io"
	"os"

	"github.com/goccy/go-yaml"

	"github.com/rusq/dlog"
)

type Config struct {
	ServerURL   string       `yaml:"server_url"` // server url for results callback
	Cert        string       `yaml:"cert"`
	Key         string       `yaml:"key"`
	ResultsDir  string       `yaml:"results_dir"`
	Deployments []Deployment `yaml:"deployments"`
}

type Deployment struct {
	Type     string      `yaml:"type"`
	Disabled bool        `yaml:"disabled"`
	Workdir  string      `yaml:"work_dir"`
	Command  []string    `yaml:"command"`
	Payload  interface{} `yaml:"payload"`
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

func (c *Config) validate() error {
	for i := range c.Deployments {
		c.Deployments[i].initOrDisable()
	}
	if c.IsEmpty() {
		return errors.New("all configurations are invalid or empty config")
	}
	return nil
}

func (m *Deployment) initOrDisable() {
	if m.Disabled {
		return
	}
	fi, err := os.Stat(m.Workdir)
	if err != nil {
		m.Disabled = true
		dlog.Printf("workdir error for %q: %s", m.Type, err)
		return
	}
	if !fi.IsDir() {
		m.Disabled = true
		dlog.Printf("[%s] %s is not a directory", m.Type, m.Workdir)
		return
	}
	if m.Payload == nil {
		m.Disabled = true
		dlog.Printf("no payload for %q deployment in %q", m.Type, m.Workdir)
		return
	}

	dp, ok := deploymentTypes[m.Type]
	if !ok {
		m.Disabled = true
		dlog.Printf("unregistered deployment type %q for workdir %q", m.Type, m.Workdir)
		return
	}
	if err := dp.Register(*m); err != nil {
		m.Disabled = true
		dlog.Printf("unable to register deployment type %q in workdir %q: %s", m.Type, m.Workdir, err)
		return
	}
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
