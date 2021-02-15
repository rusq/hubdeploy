package deploysrv

import "testing"

func TestServer_resultsURL(t *testing.T) {
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
		{"ok", fields{resultsDir: "/res/", url: "https://endless.lol"}, "https://endless.lol/" + results + "/"},
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
