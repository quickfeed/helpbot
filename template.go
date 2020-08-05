package main

import (
	"fmt"
	"text/template"

	"github.com/andersfylling/disgord"
)

var funcMap = template.FuncMap{
	"ch": func(id disgord.Snowflake) string { return fmt.Sprintf("<#%d>", id) },
}

func createTemplate(name, tmpl string) *template.Template {
	return template.Must(template.New(name).Funcs(funcMap).Parse(tmpl))
}
