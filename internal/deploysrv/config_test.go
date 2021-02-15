package deploysrv

import "testing"

func TestConfig_IsEmpty(t *testing.T) {
	type fields struct {
		Cert        string
		Key         string
		Deployments []Deployment
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{"empty", fields{}, true},
		{"empty deployments",
			fields{
				Deployments: []Deployment{},
			},
			true,
		},
		{
			name: "disabled deployment",
			fields: fields{
				Deployments: []Deployment{
					{Disabled: true},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				Cert:        tt.fields.Cert,
				Key:         tt.fields.Key,
				Deployments: tt.fields.Deployments,
			}
			if got := c.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_validate(t *testing.T) {
	type fields struct {
		Cert        string
		Key         string
		Deployments []Deployment
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{"empty repo name",
			fields{Deployments: []Deployment{
				{RepoName: ""},
			}},
			true,
		},
		{"disabled",
			fields{Deployments: []Deployment{
				{
					RepoName: "test",
					Disabled: true,
				},
			}},
			true,
		},
		{"dir does not exist",
			fields{Deployments: []Deployment{
				{
					RepoName: "test",
					Workdir:  "xxx",
				},
			}},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				Cert:        tt.fields.Cert,
				Key:         tt.fields.Key,
				Deployments: tt.fields.Deployments,
			}
			if err := c.validate(); (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
