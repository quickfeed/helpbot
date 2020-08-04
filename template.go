package main

import (
	"fmt"
	"text/template"
)

var funcMap = template.FuncMap{
	"ch": func(id string) string { return fmt.Sprintf("<#%s>", id) },
}

func createTemplate(name, tmpl string) *template.Template {
	return template.Must(template.New(name).Funcs(funcMap).Parse(tmpl))
}
