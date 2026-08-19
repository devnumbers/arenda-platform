package mailer

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"
)

// Renderer renders named email templates into plain text and HTML bodies.
type Renderer struct {
	html map[string]*template.Template
	text map[string]*texttemplate.Template
}

// NewRenderer walks dir and parses all .html and .txt email templates.
// HTML templates are wrapped with layout.html; text templates are standalone.
func NewRenderer(dir string) (_ *Renderer, err error) {
	base, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve template directory: %w", err)
	}
	base = filepath.Clean(base)

	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, fmt.Errorf("open template directory: %w", err)
	}
	defer func() {
		// The close error is folded in only when parsing succeeded, so it
		// never masks the original failure.
		if closeErr := root.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close template directory: %w", closeErr)
		}
	}()

	layoutBytes, err := readRootFile(root, "layout.html")
	if err != nil {
		return nil, fmt.Errorf("read layout.html: %w", err)
	}

	html := make(map[string]*template.Template)
	text := make(map[string]*texttemplate.Template)

	err = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		name := strings.TrimSuffix(rel, filepath.Ext(rel))

		switch strings.ToLower(filepath.Ext(rel)) {
		case ".html":
			if rel == "layout.html" {
				return nil
			}

			contentBytes, err := readRootFile(root, rel)
			if err != nil {
				return fmt.Errorf("read %s: %w", rel, err)
			}

			tmpl, err := template.New("layout").Parse(string(layoutBytes))
			if err != nil {
				return fmt.Errorf("parse layout.html for %s: %w", rel, err)
			}
			if _, err := tmpl.New("content").Parse(string(contentBytes)); err != nil {
				return fmt.Errorf("parse %s: %w", rel, err)
			}
			if _, err := tmpl.New(name).Parse(`{{template "layout" .}}`); err != nil {
				return fmt.Errorf("create wrapper for %s: %w", rel, err)
			}
			html[name] = tmpl

		case ".txt":
			contentBytes, err := readRootFile(root, rel)
			if err != nil {
				return fmt.Errorf("read %s: %w", rel, err)
			}
			tmpl, err := texttemplate.New(name).Parse(string(contentBytes))
			if err != nil {
				return fmt.Errorf("parse %s: %w", rel, err)
			}
			text[name] = tmpl
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk template directory: %w", err)
	}

	return &Renderer{html: html, text: text}, nil
}

// readRootFile reads a file by a path relative to the template root.
func readRootFile(root *os.Root, name string) (data []byte, err error) {
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() {
		// The close error is folded in only when the read succeeded, so it
		// never masks the read failure.
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()
	return io.ReadAll(f)
}

// Render executes the named template and returns the plain text and HTML bodies.
// If only one variant exists, the missing one is returned as an empty string.
func (r *Renderer) Render(name string, data any) (plain, html string, err error) {
	if r == nil {
		return "", "", errors.New("nil renderer")
	}

	if tmpl, ok := r.html[name]; ok {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			return "", "", fmt.Errorf("execute html template %q: %w", name, err)
		}
		html = buf.String()
	}

	if tmpl, ok := r.text[name]; ok {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return "", "", fmt.Errorf("execute text template %q: %w", name, err)
		}
		plain = buf.String()
	}

	if html == "" && plain == "" {
		return "", "", fmt.Errorf("template %q not found", name)
	}

	return plain, html, nil
}
