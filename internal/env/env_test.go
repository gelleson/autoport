package env

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractPortKeys(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantKeys []string
	}{
		{
			name: "basic env",
			content: `PORT=8080
API_PORT=9090
app_port=3333
OTHER_VAR=123
# DB_PORT=5432
`,
			wantKeys: []string{"API_PORT", "PORT", "app_port"},
		},
		{
			name: "spaces around equals",
			content: `
  PORT = 8080
	WEB_PORT=3000
   INVALID
`,
			wantKeys: []string{"PORT", "WEB_PORT"},
		},
		{
			name:     "empty file",
			content:  "",
			wantKeys: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.content)
			got := ExtractPortKeys(r)
			if !reflect.DeepEqual(got, tt.wantKeys) && !(len(got) == 0 && len(tt.wantKeys) == 0) {
				t.Errorf("ExtractPortKeys() = %v, want %v", got, tt.wantKeys)
			}
		})
	}
}

func TestParse(t *testing.T) {
	r := strings.NewReader(`
MONITORING_URL="http://localhost:31413/rpc"
app_port=3000
`)
	got := Parse(r)
	if got["MONITORING_URL"] != "http://localhost:31413/rpc" {
		t.Fatalf("unexpected MONITORING_URL value: %q", got["MONITORING_URL"])
	}
	if got["app_port"] != "3000" {
		t.Fatalf("unexpected app_port value: %q", got["app_port"])
	}
}
