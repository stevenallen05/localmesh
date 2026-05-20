package render

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRewriteRelativePaths(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		prefix string
		want   string // YAML excerpt that must appear in output
		unwant string // YAML excerpt that must NOT appear
	}{
		{
			name:   "build context string form",
			input:  "services:\n  app:\n    build: ./app\n",
			prefix: "service_catalog/x",
			want:   "build: service_catalog/x/app",
		},
		{
			name:   "build context mapping form",
			input:  "services:\n  app:\n    build:\n      context: ./app\n      dockerfile: Dockerfile\n",
			prefix: "service_catalog/x",
			want:   "context: service_catalog/x/app",
		},
		{
			name:   "short-form volume with mode suffix",
			input:  "services:\n  app:\n    volumes:\n      - ./data:/var/lib/data:ro\n",
			prefix: "service_catalog/x",
			want:   "- service_catalog/x/data:/var/lib/data:ro",
		},
		{
			name:   "absolute volume left alone",
			input:  "services:\n  app:\n    volumes:\n      - /etc/hosts:/etc/hosts:ro\n",
			prefix: "service_catalog/x",
			want:   "- /etc/hosts:/etc/hosts:ro",
			unwant: "service_catalog/x/etc/hosts",
		},
		{
			name:   "named volume left alone",
			input:  "services:\n  app:\n    volumes:\n      - mydata:/var/lib/data\n",
			prefix: "service_catalog/x",
			want:   "- mydata:/var/lib/data",
		},
		{
			name:   "configs file rewritten",
			input:  "configs:\n  cfg:\n    file: ./cfg.yml\n",
			prefix: "service_catalog/x",
			want:   "file: service_catalog/x/cfg.yml",
		},
		{
			name:   "secrets file rewritten",
			input:  "secrets:\n  s1:\n    file: ./secret.txt\n",
			prefix: "service_catalog/x",
			want:   "file: service_catalog/x/secret.txt",
		},
		{
			name:   "dotdot parent path rewritten",
			input:  "services:\n  app:\n    volumes:\n      - ../shared:/run/shared:ro\n",
			prefix: "service_catalog/x",
			want:   "- service_catalog/shared:/run/shared:ro",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.input), &doc); err != nil {
				t.Fatalf("parse input: %v", err)
			}
			RewriteRelativePaths(&doc, tt.prefix)
			out, err := yaml.Marshal(&doc)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !contains(string(out), tt.want) {
				t.Errorf("want substring %q in:\n%s", tt.want, out)
			}
			if tt.unwant != "" && contains(string(out), tt.unwant) {
				t.Errorf("did NOT want %q in:\n%s", tt.unwant, out)
			}
		})
	}
}

func TestIsRelativePath(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"./foo", true},
		{"../foo", true},
		{".", true},
		{"..", true},
		{"/abs/path", false},
		{"mydata", false},
		{"image:tag", false},
		{"", false},
	}
	for _, c := range cases {
		got := isRelativePath(c.in)
		if got != c.want {
			t.Errorf("isRelativePath(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
