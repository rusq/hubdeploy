package deploysrv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/rusq/dlog"
)

const (
	defJobQueueSz = 100
	resultExt     = ".txt"
	stall         = 991 * time.Millisecond
)

type Server struct {
	cert    string
	privkey string

	mapping map[string]Deployment
	jobs    chan job
	results chan result

	url        string
	resultsDir string
	prefix     string
}

type job struct {
	wh DockerhubWebhook
	d  Deployment
}

type result struct {
	id     uuid.UUID
	output []byte
	err    error
}

type Option func(*Server)

func OptWithCert(cert, privkey string) Option {
	return func(s *Server) {
		if cert == "" || privkey == "" {
			return
		}
		s.cert, s.privkey = cert, privkey
	}
}

func OptWithPrefix(prefix string) Option {
	return func(s *Server) {
		s.prefix = prefix
	}
}

func OptWithResultDir(dir string) Option {
	return func(s *Server) {
		s.resultsDir = dir
	}
}

func New(c Config, opts ...Option) (*Server, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	s := &Server{
		mapping:    c.repomap(),
		cert:       c.Cert,
		privkey:    c.Key,
		resultsDir: c.ResultsDir,
		results:    make(chan result),
		jobs:       make(chan job, defJobQueueSz),
		url:        c.FQDN,
	}

	for _, opt := range opts {
		opt(s)
	}
	if s.resultsDir != "" {
		if err := s.initResultDir(); err != nil {
			return nil, err
		}
	}

	go s.dispatcher(s.results, s.jobs)
	go s.printer(s.results)

	return s, nil
}

func (s *Server) initResultDir() error {
	absPath, err := filepath.Abs(s.resultsDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return err
	}
	s.resultsDir = absPath
	return nil
}

func (s *Server) ListenAndServe(addr string) error {
	defer close(s.jobs)
	mux := s.routes()
	mux = logMiddleware(mux)
	if s.cert != "" && s.privkey != "" {
		dlog.Debugln("TLS enabled")
		return http.ListenAndServeTLS(addr, s.cert, s.privkey, mux)
	} else {
		return http.ListenAndServe(addr, mux)
	}
}

func (s *Server) dispatcher(results chan<- result, jobs <-chan job) {
	for j := range jobs {
		id, output, err := s.deploy(j.wh, j.d)
		results <- result{id: id, output: output, err: err}
	}
}

func (s *Server) printer(results <-chan result) {
	for res := range results {
		msg := "OK"
		if res.err != nil {
			msg = res.err.Error()
		}
		dlog.Printf("%s>  result:  %s", res.id, msg)
		go s.maybeSave(res.id, res.output)
	}
}

func (s *Server) deploy(wh DockerhubWebhook, d Deployment) (uuid.UUID, []byte, error) {
	state := StateSuccess
	descr := "OK"

	id, output, err := s.runDeployment(wh, d)
	if err != nil {
		state = StateError
		descr = err.Error()
	}
	if e := s.sendCallback(wh.CallbackURL, id.String(), state, descr); e != nil {
		dlog.Printf("failed to send callback: %s", e)
	}
	return id, output, err
}

func (*Server) runDeployment(wh DockerhubWebhook, d Deployment) (uuid.UUID, []byte, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return uuid.UUID{}, nil, err
	}
	defer os.Chdir(cwd)

	t := time.Unix(int64(wh.PushData.PushedAt), 0)
	id := uuid.Must(uuid.NewUUID())
	dlog.Printf("%s> starting deployment for %s in %s (image pushed at %s)", id.String(), d.RepoName, d.Workdir, t)

	if err := os.Chdir(d.Workdir); err != nil {
		return id, nil, fmt.Errorf("%s> chdir to %s failed: %w", id.String(), d.Workdir, err)
	}
	command, args := head(d.Command...)
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return id, output, fmt.Errorf("%s> execution failed with %w: %s", id.String(), err, string(output))
	}
	dlog.Debugln(string(output))
	dlog.Printf("%s> completed without errors.", id)
	return id, output, nil
}

func (s *Server) sendCallback(url string, id string, state string, description string) error {
	cb := DockerHubCallback{
		State:       state,
		Description: fmt.Sprintf("[%s]: %s", id, description),
		Context:     "Continuous integration by github.com/rusq/hubdeploy",
	}
	if s.url != "" {
		cb.TargetURL = s.url + path.Join(s.prefix, results) + "/" + id + resultExt
	}
	data, err := json.Marshal(cb)
	if err != nil {
		return err
	}
	// post the results
	dlog.Printf("%s> posting results to %s", id, url)
	dlog.Debugf("%s> data: %s", id, string(data))
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid status code: %d", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	dlog.Printf("%s> post ok", id)
	dlog.Debugf("%s> body: %s", id, string(body))
	return nil
}

func (s *Server) maybeSave(id uuid.UUID, output []byte) {
	if s.resultsDir == "" {
		return
	}
	if err := ioutil.WriteFile(filepath.Join(s.resultsDir, id.String()+resultExt), output, 0600); err != nil {
		dlog.Println(err)
	}
	return
}

func head(s ...string) (string, []string) {
	if len(s) == 0 {
		return "", nil
	}
	if len(s) == 1 {
		return s[0], nil
	}
	return s[0], s[1:]
}
