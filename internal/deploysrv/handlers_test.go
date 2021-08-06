package deploysrv

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServer_resultsHandler(t *testing.T) {

	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempdir)
	testfile, err := ioutil.TempFile(tempdir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer testfile.Close()

	const testContents = "test file contents"
	if _, err := testfile.Write([]byte(testContents)); err != nil {
		t.Fatal(err)
	}
	testfile.Close()

	type fields struct {
		cert       string
		privkey    string
		jobs       chan Job
		results    chan result
		url        string
		resultsDir string
		prefix     string
	}
	type args struct {
		w http.ResponseWriter
		r *http.Request
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantCode int
		wantBody string
	}{
		{
			name:   "no results dir",
			fields: fields{resultsDir: ""},
			args: args{
				httptest.NewRecorder(),
				httptest.NewRequest(http.MethodGet, "/file.txt", nil),
			},
			wantCode: http.StatusNotFound,
			wantBody: "404 page not found\n",
		},
		{
			name:   "no filename",
			fields: fields{resultsDir: "results"},
			args: args{
				httptest.NewRecorder(),
				httptest.NewRequest(http.MethodGet, "/dir/", nil),
			},
			wantCode: http.StatusNotFound,
			wantBody: "404 page not found\n",
		},
		{
			name:   "not exist",
			fields: fields{resultsDir: "results"},
			args: args{
				httptest.NewRecorder(),
				httptest.NewRequest(http.MethodGet, "/dir/xxx", nil),
			},
			wantCode: http.StatusNotFound,
			wantBody: "404 page not found\n",
		},
		{
			name:   "existing file",
			fields: fields{resultsDir: tempdir},
			args: args{
				httptest.NewRecorder(),
				httptest.NewRequest(http.MethodGet, "/"+testfile.Name(), nil),
			},
			wantCode: http.StatusOK,
			wantBody: testContents,
		},
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

			s.resultsHandler(tt.args.w, tt.args.r)

			rec := tt.args.w.(*httptest.ResponseRecorder)

			assert.Equal(t, tt.wantCode, rec.Code)
			assert.Equal(t, tt.wantBody, rec.Body.String())
		})
	}
}
