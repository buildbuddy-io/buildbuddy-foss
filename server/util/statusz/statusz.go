package statusz

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/buildbuddy-io/buildbuddy/server/version"
)

const (
	BasePath = "/statusz"

	Route = "/statusz/{section_name...}"

	templateContents = `<!DOCTYPE html>
<html>
  <head>
    <title>/statusz</title>
    <style>
      body {
	font-family: sans-serif;
      }
      h1 {
	width: 100%;
	text-align: center;
	font-size: 120%;
	background: #eeeeff;
      }
      h2 {
	width: 100%;
	font-size: 110%;
	text-align: center;
	background: #fffddd;
      }
      .header {
	display: flex;
	justify-content: space-between;
	padding-bottom: 48px
      }
    </style>
  </head>
  <h1>Status for {{.BinaryName}}</h1>
  <div class="header">
    <div>
      <div>Started at {{.StartTime.Format "Jan 02, 2006 15:04:05 MST" }}</div>
      <div>Current time {{.CurrentTime.Format "Jan 02, 2006 15:04:05 MST" }}</div>
      <div>App Version {{.AppVersion}} ({{.Commit}})</div>
      <div>Go Version {{.GoVersion}}</div>
    </div>
    <div>
      {{.Username}}@{{.Hostname}}
    </div>
  </div>
  {{range $idx, $item := .Sections}}
  <div>
    <h2>
      {{$item.Name}}
    </h2>
    <div>
      <div>{{$item.Description}}</div>
      <div>{{$item.HTML}}</div>
    </div>
  </div>
  {{end}}
</html>
`
)

var (
	startTime          time.Time
	hostname           string
	username           string
	binaryName         = filepath.Base(os.Args[0])
	statusPageTemplate = template.Must(template.New("statusz").Parse(templateContents))
	DefaultHandler     = NewHandler()
)

func init() {
	startTime = time.Now()
	if h, err := os.Hostname(); err == nil {
		hostname = h
	}
	if u, err := user.Current(); err == nil {
		username = u.Username
	}
}

type StatusReporter interface {
	// Returns a text or HTML snippet that represents the service's current
	// running state in a compact way.
	Statusz(ctx context.Context) string
}

type StatusServer interface {
	// Sections that implement this interface may implement arbitrary additional
	// HTTP serving logic, e.g. to implement POST updates or expose artifacts
	// referenced in the statusz sections. Requests are only routed to these
	// handlers if they contain a path starting with "/statusz/{section_name}/".
	// The "/statusz/{section_name}" prefix is stripped.
	ServeStatusz(w http.ResponseWriter, r *http.Request)
}

type StatusFunc func(ctx context.Context) string
type ServeFunc = http.HandlerFunc

type Section struct {
	Name        string
	Description string
	Status      StatusFunc
	Serve       ServeFunc
}

type Handler struct {
	mu       sync.RWMutex // PROTECTS(sections)
	sections map[string]*Section
}

func NewHandler() *Handler {
	return &Handler{
		mu:       sync.RWMutex{},
		sections: make(map[string]*Section, 0),
	}
}

func (h *Handler) AddSection(name, description string, statusReporter StatusReporter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := &Section{
		Name:        name,
		Description: description,
		Status:      statusReporter.Statusz,
	}
	if sm, ok := statusReporter.(StatusServer); ok {
		s.Serve = http.StripPrefix(BasePath+"/"+name, http.HandlerFunc(sm.ServeStatusz)).ServeHTTP
	}
	h.sections[name] = s
}

type renderedSection struct {
	Name        string
	Description string
	HTML        template.HTML
}

func (h *Handler) renderSections(sections []*renderedSection, w http.ResponseWriter) {
	sort.Slice(sections, func(i, j int) bool {
		return sections[i].Name < sections[j].Name
	})

	data := struct {
		BinaryName  string
		Commit      string
		Username    string
		Hostname    string
		StartTime   time.Time
		CurrentTime time.Time
		AppVersion  string
		GoVersion   string
		Sections    []*renderedSection
	}{
		BinaryName:  binaryName,
		Commit:      version.Commit(),
		Username:    username,
		Hostname:    hostname,
		StartTime:   startTime,
		CurrentTime: time.Now(),
		AppVersion:  version.Tag(),
		GoVersion:   version.GoVersion(),
		Sections:    sections,
	}
	statusPageTemplate.Execute(w, data)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ctx := r.Context()

	sectionName, _, _ := strings.Cut(r.PathValue("section_name"), "/")
	if section, ok := h.sections[sectionName]; ok {
		if section.Serve != nil {
			section.Serve(w, r)
			return
		}
		http.Error(w, fmt.Sprintf("section %q does not support HTTP handling", sectionName), http.StatusBadRequest)
		return
	}

	// If no single-section was specified, render all sections.
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sections := make([]*renderedSection, 0, len(h.sections))
	for _, section := range h.sections {
		if sectionName != "" && section.Name != sectionName {
			continue
		}
		rs := &renderedSection{
			Name:        section.Name,
			Description: section.Description,
			HTML:        template.HTML(section.Status(ctx)),
		}
		sections = append(sections, rs)
	}
	h.renderSections(sections, w)
}

func AddSection(name, description string, statusReporter StatusReporter) {
	DefaultHandler.AddSection(name, description, statusReporter)
}

func Server() http.Handler {
	return DefaultHandler
}
