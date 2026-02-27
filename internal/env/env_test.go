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
OTHER_VAR=123
# DB_PORT=5432
`,
			wantKeys: []string{"PORT", "API_PORT"},
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
