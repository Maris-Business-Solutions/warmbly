package tmplfuncs

import (
	"errors"
	"strings"
	"testing"
	"text/template"
)

func mustParse(t *testing.T, src string) *template.Template {
	t.Helper()
	tmpl, err := template.New("t").Funcs(FuncMap()).Option("missingkey=zero").Parse(src)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return tmpl
}

func TestCheckLimitsRefusesUnboundedConstructs(t *testing.T) {
	for _, src := range []string{
		`{{range 100000000000}}x{{end}}`,
		`{{range (len .name)}}x{{end}}`,
		`{{range len .name}}x{{end}}`,
		`{{$n := 100000000000}}{{range $n}}x{{end}}`,
		`{{with 100000000000}}{{range .}}x{{end}}{{end}}`,
		`{{with $n := 100000000000}}{{range .}}x{{end}}{{end}}`,
		`{{$ = 100000000000}}{{range $}}x{{end}}`,
		`{{range .a}}{{range .b}}{{range .c}}x{{end}}{{end}}{{end}}`,
		`{{define "a"}}{{template "a"}}{{template "a"}}{{end}}{{template "a"}}`,
		`{{block "b" .}}x{{end}}`,
	} {
		if err := CheckLimits(mustParse(t, src)); err == nil {
			t.Errorf("CheckLimits(%q) = nil, want a refusal", src)
		}
	}
}

func TestCheckLimitsAllowsDataTemplates(t *testing.T) {
	for _, src := range []string{
		`Hi {{.FirstName | default "there"}}`,
		`{{if eq .Company "Acme"}}a{{else if .Phone}}b{{else}}c{{end}}`,
		`{{range .items}}{{.name}}{{end}}`,
		`{{range $i, $x := .items}}{{$i}}={{$x.name}}{{end}}`,
		`{{range .lead.tags}}{{.}}{{else}}none{{end}}`,
		`{{range $.items}}{{range .tags}}{{.}}{{end}}{{end}}`,
		`{{with .lead}}{{range .tags}}{{.}}{{end}}{{end}}`,
		`{{range $k, $v := .}}{{$k}}{{end}}`,
		`{{$x := .items}}{{range $x.list}}{{.}}{{end}}`,
	} {
		if err := CheckLimits(mustParse(t, src)); err != nil {
			t.Errorf("CheckLimits(%q) = %v, want nil", src, err)
		}
	}
}

func TestExecuteCapsOutput(t *testing.T) {
	big := strings.Repeat("x", MaxOutput/4+1)
	data := map[string]any{"items": []string{big, big, big, big, big}}
	if _, err := Execute(mustParse(t, `{{range .items}}{{.}}{{end}}`), data); !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("Execute over the cap: err = %v, want ErrOutputLimit", err)
	}
	out, err := Execute(mustParse(t, `Hi {{.name}}`), map[string]any{"name": "Ann"})
	if err != nil || out != "Hi Ann" {
		t.Fatalf("Execute = %q, %v", out, err)
	}
}
