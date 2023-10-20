package server

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"
	"sync"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/labstack/echo/v4"
)

type Template struct {
	reload     bool
	rootPath   string
	templates  *template.Template
	lock       *sync.Mutex
	buildCache map[string]string
}

func NewTemplateRenderer(rootPath string, reload bool) *Template {
	return &Template{
		reload:     reload,
		rootPath:   rootPath,
		templates:  nil,
		lock:       &sync.Mutex{},
		buildCache: map[string]string{},
	}
}

func (t *Template) Render(w io.Writer, name string, data any, c echo.Context) error {
	t.lock.Lock()
	if t.reload || t.templates == nil {
		var err error
		t.templates, err = template.New("templates").Funcs(template.FuncMap{
			"build": t.ESBuild,
		}).ParseGlob(t.rootPath + "templates/*")
		if err != nil {
			return err
		}
	}
	t.lock.Unlock()
	return t.templates.ExecuteTemplate(w, name, data)
}

func (t *Template) ESBuild(asset string, tag string) (any, error) {
	if !t.reload && t.buildCache[asset] != "" {
		return t.buildCache[asset], nil
	}
	result := esbuild.Build(esbuild.BuildOptions{
		EntryPoints: []string{t.rootPath + asset},
		Outfile:     "out",
		Bundle:      true,
		Format:      esbuild.FormatESModule,
		External:    []string{"*.jpg"},
	})
	issues := []string{}

	issues = append(issues, esbuild.FormatMessages(result.Errors, esbuild.FormatMessagesOptions{
		TerminalWidth: 120,
		Kind:          esbuild.ErrorMessage,
		Color:         false,
	})...)

	for _, msg := range esbuild.FormatMessages(result.Errors, esbuild.FormatMessagesOptions{
		TerminalWidth: 120,
		Kind:          esbuild.ErrorMessage,
		Color:         true,
	}) {
		fmt.Fprintf(os.Stderr, msg+"\n")
	}

	for _, msg := range esbuild.FormatMessages(result.Warnings, esbuild.FormatMessagesOptions{
		TerminalWidth: 120,
		Kind:          esbuild.WarningMessage,
		Color:         true,
	}) {
		fmt.Fprintf(os.Stderr, msg+"\n")
	}

	output := ""
	if len(result.OutputFiles) == 1 {
		output = string(result.OutputFiles[0].Contents)
	} else {
		issues = append(issues, "failed to compile: got empty output")
	}
	if len(issues) > 0 {
		return "", fmt.Errorf(strings.Join(issues, "\n"))
	}
	if tag != "" {
		output = "<" + tag + ">\n" + output + "\n</" + tag + ">"
	}
	t.buildCache[asset] = output
	return template.HTML(output), nil
}
