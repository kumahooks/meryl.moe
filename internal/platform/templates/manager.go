// Package templates manages HTML template parsing and rendering.
//
// In production (dev=false), templates are served from the embedded FS baked
// into the binary at compile time
// In dev mode (dev=true), templates are read from disk on every request so
// file changes are picked up without restarting
package templates

import (
	fmt "fmt"
	template "html/template"
	fs "io/fs"
	log "log"
	http "net/http"
	os "os"
	sync "sync"
)

// Manager holds the base template set (layouts + components) and an fs.FS
// pointing to the template root
// prod: embedded FS
// dev: os.DirFS
type Manager struct {
	baseTemplate   *template.Template
	fileSystem     fs.FS
	pageCache      map[string]*template.Template
	pageCacheMutex sync.RWMutex
	isDevelopment  bool
}

func NewManager(isDevelopment bool) (*Manager, error) {
	var fileSystem fs.FS

	if isDevelopment {
		fileSystem = os.DirFS("internal/platform/templates")
	} else {
		fileSystem = templateFS
	}

	base := template.New("")

	if _, err := base.ParseFS(fileSystem, "layouts/*.html"); err != nil {
		return nil, err
	}

	if _, err := base.ParseFS(fileSystem, "components/*.html"); err != nil {
		return nil, err
	}

	return &Manager{
		baseTemplate:  base,
		fileSystem:    fileSystem,
		pageCache:     make(map[string]*template.Template),
		isDevelopment: isDevelopment,
	}, nil
}

// Render executes the page template against the request, returning either a
// full page or a fragment depending on HTMX headers.
//
// Every page template must define {{define "page-content"}}.
func (templateManager *Manager) Render(
	writer http.ResponseWriter,
	request *http.Request,
	pageFile string,
	data any,
) error {
	pageTemplate, err := templateManager.resolve(pageFile)
	if err != nil {
		return err
	}

	// hx-boost sends both HX-Request and HX-Boosted - return full page so
	// HTMX can merge <head> (CSS, title) and swap <body>.
	isFragment := request.Header.Get("HX-Request") == "true" && request.Header.Get("HX-Boosted") != "true"
	if isFragment {
		return pageTemplate.ExecuteTemplate(writer, "page-content", data)
	}

	return pageTemplate.ExecuteTemplate(writer, "base", data)
}

func (templateManager *Manager) resolve(pageFile string) (*template.Template, error) {
	if templateManager.isDevelopment {
		return templateManager.build(pageFile)
	}

	templateManager.pageCacheMutex.RLock()
	cached, ok := templateManager.pageCache[pageFile]
	templateManager.pageCacheMutex.RUnlock()
	if ok {
		return cached, nil
	}

	templateManager.pageCacheMutex.Lock()
	defer templateManager.pageCacheMutex.Unlock()

	if cached, ok = templateManager.pageCache[pageFile]; ok {
		return cached, nil
	}

	built, err := templateManager.build(pageFile)
	if err != nil {
		return nil, err
	}

	templateManager.pageCache[pageFile] = built
	return built, nil
}

func (templateManager *Manager) build(pageFile string) (*template.Template, error) {
	clonedTemplate, err := templateManager.baseTemplate.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone base template set: %w", err)
	}

	if _, err = clonedTemplate.ParseFS(templateManager.fileSystem, pageFile); err != nil {
		return nil, fmt.Errorf("failed to parse page template %q: %w", pageFile, err)
	}

	return clonedTemplate, nil
}
