package deploysrv

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rusq/dlog"
)

type stubHooker struct {
	registerErr error
	callbackErr error
	callbacks   chan CallbackData
}

func (s *stubHooker) Register(Deployment) error { return s.registerErr }
func (s *stubHooker) Handler(chan<- Job) http.HandlerFunc {
	panic("not used in tests")
}
func (s *stubHooker) Callback(data CallbackData) error {
	if s.callbacks != nil {
		s.callbacks <- data
	}
	return s.callbackErr
}
func (s *stubHooker) Type() string { return "stub" }

func withDeploymentTypes(t *testing.T) {
	t.Helper()
	old := deploymentTypes
	deploymentTypes = make(map[string]Hooker)
	t.Cleanup(func() {
		deploymentTypes = old
	})
}

func TestServer_resultsURL(t *testing.T) {
	withDeploymentTypes(t)

	type fields struct {
		cert       string
		privkey    string
		jobs       chan Job
		results    chan result
		url        string
		resultsDir string
		prefix     string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"root prefix", fields{resultsDir: "/res/", url: "https://endless.lol"}, "https://endless.lol/" + results + "/"},
		{"non-root prefix", fields{resultsDir: "/res/", url: "https://endless.lol", prefix: "/api"}, "https://endless.lol/api/" + results + "/"},
		{"prefix trailing slash", fields{resultsDir: "/res/", url: "https://endless.lol/", prefix: "/api/"}, "https://endless.lol/api/" + results + "/"},
		{"empty server url", fields{resultsDir: "/res/"}, ""},
		{"empty results dir", fields{url: "https://endless.lol", prefix: "/api"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				cert:       tt.fields.cert,
				privkey:    tt.fields.privkey,
				jobs:       tt.fields.jobs,
				results:    tt.fields.results,
				url:        tt.fields.url,
				resultsDir: tt.fields.resultsDir,
				prefix:     tt.fields.prefix,
			}
			if got := s.resultsURL(); got != tt.want {
				t.Errorf("Server.resultsURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServer_processor_logsCallbackFailures(t *testing.T) {
	withDeploymentTypes(t)

	hook := &stubHooker{
		callbackErr: errors.New("callback down"),
		callbacks:   make(chan CallbackData, 1),
	}
	deploymentTypes[hook.Type()] = hook

	var buf bytes.Buffer
	dlog.SetOutput(&buf)
	t.Cleanup(func() {
		dlog.SetOutput(os.Stderr)
	})

	s := &Server{
		resultsDir: t.TempDir(),
		url:        "https://example.test",
		prefix:     "/api",
	}

	resultsCh := make(chan result)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.processor(resultsCh)
	}()

	id := uuid.Must(uuid.NewUUID())
	resultsCh <- result{id: id, typ: hook.Type(), url: "https://callback.test", output: []byte("ok")}

	select {
	case cb := <-hook.callbacks:
		wantURL := "https://example.test/api/results/" + id.String() + resultExt
		if cb.ResultsURL != wantURL {
			t.Fatalf("CallbackData.ResultsURL = %q, want %q", cb.ResultsURL, wantURL)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("processor did not invoke callback")
	}

	close(resultsCh)
	wg.Wait()

	if !strings.Contains(buf.String(), "callback failed") {
		t.Fatalf("expected callback failure to be logged, got %q", buf.String())
	}
}

func TestNew_requiresRegisteredHookersBeforeValidation(t *testing.T) {
	withDeploymentTypes(t)

	workdir := t.TempDir()
	newConfig := func() Config {
		return Config{
			ResultsDir: filepath.Join(t.TempDir(), "results"),
			Deployments: []Deployment{
				{
					Type:    "stub",
					Workdir: workdir,
					Command: []string{"echo", "ok"},
					Payload: map[string]any{"repo": "demo"},
				},
			},
		}
	}

	if _, err := New(newConfig()); err == nil {
		t.Fatal("New() error = nil, want unregistered deployment type failure")
	}

	if err := Register(&stubHooker{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	srv, err := New(newConfig())
	if err != nil {
		t.Fatalf("New() after Register() error = %v", err)
	}
	close(srv.jobs)
	close(srv.results)
}
