package deploysrv

import (
	"errors"
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

	results = "results"
)

const (
	goAway = "get lost"
)

var deploymentTypes = map[string]Hooker{}

type Server struct {
	cert    string
	privkey string

	jobs    chan Job
	results chan result

	url        string
	resultsDir string
	prefix     string
}

type Job struct {
	CallbackURL string
	Dep         Deployment
}

// Hooker is the interface for pluggable webhook handlers.
type Hooker interface {
	// Register must register a deployment. If deployment type is different then the one handled
	// it must return nil.
	Register(Deployment) error
	// Handle must handle the incoming webhook and post a Job to a Job channel.
	Handler(chan<- Job) http.HandlerFunc
	// Callback can send (or not, if not implemented by the caller) the callback to source system
	// with the build results info.
	Callback(CallbackData) error
	// Type must return the string that will be used as deployment type.  It will also be a path of a webhook
	Type() string
}
type CallbackData struct {
	ID          uuid.UUID
	CallbackURL string
	Description string
	Context     string
	Error       error
	ResultsURL  string
}

type result struct {
	id     uuid.UUID
	output []byte
	url    string
	typ    string
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
		cert:       c.Cert,
		privkey:    c.Key,
		resultsDir: c.ResultsDir,
		results:    make(chan result),
		jobs:       make(chan Job, defJobQueueSz),
		url:        c.ServerURL,
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
	go s.processor(s.results)

	return s, nil
}

// Register allows to register custom Hookers.  Must be called after New and before ListenAndServe.
func Register(h Hooker) error {
	if h == nil {
		return errors.New("programming error:  hooker is empty")
	}
	deploymentTypes[h.Type()] = h
	return nil
}

func (s *Server) resultsURL() string {
	if s.url == "" || s.resultsDir == "" {
		return ""
	}
	if s.url[len(s.url)-1] != '/' {
		s.url = s.url + "/"
	}
	return s.url + results + "/"

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

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(path.Join(s.prefix, "/"), func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(stall * 2) // stall the motherfucker
		http.Error(w, goAway, http.StatusNotFound)
	})
	if s.resultsDir != "" {
		mux.HandleFunc(path.Join(s.prefix, results)+"/", s.resultsHandler)
	}

	s.initWebhookHandlers(mux)

	return mux
}

func (s *Server) initWebhookHandlers(mux *http.ServeMux) {
	if len(deploymentTypes) == 0 {
		dlog.Panic("no deployment handlers, don't know how we got this far")
	}
	for name, d := range deploymentTypes {
		mux.HandleFunc(path.Join(s.prefix, "webhooks", name)+"/", d.Handler(s.jobs))
	}
}

func (s *Server) dispatcher(results chan<- result, jobs <-chan Job) {
	for j := range jobs {
		id, output, err := s.runDeployment(j.Dep)
		results <- result{
			id:     id,
			output: output,
			typ:    j.Dep.Type,
			url:    j.CallbackURL,
			err:    err,
		}
	}
}

func (s *Server) processor(results <-chan result) {
	for res := range results {
		msg := "OK"
		if res.err != nil {
			msg = res.err.Error()
		}
		dlog.Printf("%s>  result:  %s", res.id, msg)

		s.maybeSave(res.id, res.output)

		dp, ok := deploymentTypes[res.typ]
		if !ok {
			dlog.Printf("*** INTERNAL ERROR***: got result for unregistered type %q", res.typ)
			continue
		}

		dp.Callback(CallbackData{
			ID:          res.id,
			CallbackURL: res.url,
			Description: ifErrNotNil(res.err, "deployed with error", "deployed OK"),
			Context:     "Continuous integration by github.com/rusq/hubdeploy",
			Error:       res.err,
			ResultsURL:  s.resultsURL() + res.id.String() + resultExt,
		})
	}
}

func ifErrNotNil(err error, whenNotNil, whenNil string) string {
	if err != nil {
		return whenNotNil
	}
	return whenNil
}

func (*Server) runDeployment(d Deployment) (uuid.UUID, []byte, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return uuid.UUID{}, nil, err
	}
	defer os.Chdir(cwd)

	id := uuid.Must(uuid.NewUUID())
	dlog.Printf("%s> starting %q deployment in %q", id.String(), d.Type, d.Workdir)

	if err := os.Chdir(d.Workdir); err != nil {
		return id, nil, fmt.Errorf("%s> chdir to %q failed: %w", id.String(), d.Workdir, err)
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
