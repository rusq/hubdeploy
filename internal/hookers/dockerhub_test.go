package hookers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/google/uuid"

	"github.com/rusq/hubdeploy/internal/deploysrv"
)

var dockerValid = `---
type: dockerhub
payload:
  repo_name: test_repo
  tags:
    - tag1
    - tag2
`

var dockerDepValid deploysrv.Deployment

func init() {
	if err := yaml.Unmarshal([]byte(dockerValid), &dockerDepValid); err != nil {
		panic(err)
	}
}

func TestDockerHub_Register(t *testing.T) {
	type fields struct {
		mapping map[string]map[string]deploysrv.Deployment
	}
	type args struct {
		dep deploysrv.Deployment
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"valid", fields{}, args{dockerDepValid}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DockerHub{
				mapping: tt.fields.mapping,
			}
			if err := d.Register(tt.args.dep); (err != nil) != tt.wantErr {
				t.Errorf("DockerHub.Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDockerHub_Callback(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{name: "ok", statusCode: http.StatusOK},
		{name: "bad status", statusCode: http.StatusBadGateway, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got callback
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Fatalf("decode callback body: %v", err)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			d := &DockerHub{}
			err := d.Callback(deploysrv.CallbackData{
				ID:          uuid.Must(uuid.NewUUID()),
				CallbackURL: srv.URL,
				Description: "deployed OK",
				Context:     "test context",
				ResultsURL:  "https://example.test/results/id.txt",
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("Callback() error = %v, wantErr %v", err, tt.wantErr)
			}

			if got.Context != "test context" {
				t.Fatalf("callback context = %q, want %q", got.Context, "test context")
			}
			if got.TargetURL != "https://example.test/results/id.txt" {
				t.Fatalf("callback target_url = %q, want %q", got.TargetURL, "https://example.test/results/id.txt")
			}
			if got.State != ssuccess {
				t.Fatalf("callback state = %q, want %q", got.State, ssuccess)
			}
		})
	}
}
