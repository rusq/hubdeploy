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
	// Register must register a deployment. If deployment type is different then
	// the one handled it must return nil.
	Register(Deployment) error
	// Handle must handle the incoming webhook and post a Job to a Job channel.
	Handler(chan<- Job) http.HandlerFunc
	// Callback can send (or not, if not implemented by the caller) the callback
	// to source system with the build results info.
	Callback(CallbackData) error
	// Type must return the string that will be used as deployment type.  It
	// will also be a path of a webhook.
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

// OptWithCert allows to specify the certificate and the private key for TLS
// Listener.
func OptWithCert(cert, privkey string) Option {
	return func(s *Server) {
		if cert == "" || privkey == "" {
			return
		}
		s.cert, s.privkey = cert, privkey
	}
}

// OptWithPrefix allows to set the url prefix for the muxer.
func OptWithPrefix(prefix string) Option {
	return func(s *Server) {
		s.prefix = prefix
	}
}

// OptWithResultDir sets the directory which will contain the results of
// deployment (combined STDOUT and STDERR outputs).
func OptWithResultDir(dir string) Option {
	return func(s *Server) {
		s.resultsDir = dir
	}
}

// New constructs new hubdeploy server instance.
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

// Register allows to register custom Hookers.  Must be called after New and
// before ListenAndServe.
func Register(h Hooker) error {
	if h == nil {
		return errors.New("programming error:  hooker is empty")
	}
	deploymentTypes[h.Type()] = h
	return nil
}

// resultsURL resolves the results URL.
func (s *Server) resultsURL() string {
	if s.url == "" || s.resultsDir == "" {
		return ""
	}
	if s.url[len(s.url)-1] != '/' {
		s.url = s.url + "/"
	}
	return s.url + results + "/"

}

// initResultDir checks if the directory exists, if not - creates it.
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

// ListenAndServe listens for incoming connections on the specified address and
// Serves them.
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

// routes creates handlers for the url paths.
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

// initWebhookHandlers initialises webhooks handlers.
func (s *Server) initWebhookHandlers(mux *http.ServeMux) {
	if len(deploymentTypes) == 0 {
		dlog.Panic("no deployment handlers, don't know how we got this far")
	}
	for name, d := range deploymentTypes {
		mux.HandleFunc(path.Join(s.prefix, "webhooks", name)+"/", d.Handler(s.jobs))
	}
}

// dispatcher runs the deployments and sends the results to the results chan.
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

// processor processes results.
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
			dlog.Printf("*** INTERNAL ERROR***: got result for unregistered deployment type %q", res.typ)
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

// ifErrNotNil returns a predefined string depending on the error value.
func ifErrNotNil(err error, whenNotNil, whenNil string) string {
	if err != nil {
		return whenNotNil
	}
	return whenNil
}

// runDeployment runs the deployment, returning the result UUID, deployment
// output and an error.
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

// maybeSave maybe saves output to the file with UUID as name and resultExt as
// an extension.
func (s *Server) maybeSave(id uuid.UUID, output []byte) {
	if s.resultsDir == "" {
		return
	}
	if err := ioutil.WriteFile(filepath.Join(s.resultsDir, id.String()+resultExt), output, 0600); err != nil {
		dlog.Println(err)
	}
}

// head splits the slice returning first element and the rest as a slice.
func head(s ...string) (string, []string) {
	if len(s) == 0 {
		return "", nil
	}
	if len(s) == 1 {
		return s[0], nil
	}
	return s[0], s[1:]
}
