package deploysrv

import (
	"testing"

	"github.com/goccy/go-yaml"
)

var dockerValid = `---
type: dockerhub
payload:
  repo_name: test_repo
  tags:
    - tag1
    - tag2
`

var dockerDepValid Deployment

func init() {
	if err := yaml.Unmarshal([]byte(dockerValid), &dockerDepValid); err != nil {
		panic(err)
	}
}

func TestDockerHub_Register(t *testing.T) {
	type fields struct {
		mapping map[string]map[string]Deployment
	}
	type args struct {
		dep Deployment
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
