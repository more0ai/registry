// Package server orchestrates all components: NATS client, DB, registry, dispatcher, HTTP health.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	comms "github.com/nats-io/nats.go"

	"github.com/morezero/capabilities-registry/internal/config"
	"github.com/morezero/capabilities-registry/pkg/bootstrap"
	"github.com/morezero/capabilities-registry/pkg/db"
	"github.com/morezero/capabilities-registry/pkg/dispatcher"
	"github.com/morezero/capabilities-registry/pkg/events"
	"github.com/morezero/capabilities-registry/pkg/commsutil"
	"github.com/morezero/capabilities-registry/pkg/registry"
)

const logPrefix = "server:server"

// Server is the capabilities-registry orchestrator.
type Server struct {
	cfg        *config.Config
	nc         *comms.Conn
	pool       *pgxpool.Pool
	httpServer *http.Server
	reg        *registry.Registry
}

// Run starts the server, blocks until shutdown signal, then cleans up.
func Run() error {
	// Setup structured logging
	var logLevel slog.Level
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("%s - failed to load config: %w", logPrefix, err)
	}

	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	slog.Info(fmt.Sprintf("%s - Starting capabilities-registry", logPrefix))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := &Server{cfg: cfg}

	// Step 1: Load bootstrap config
	bootstrapCfg, err := bootstrap.LoadBootstrapConfig(cfg.BootstrapFile)
	if err != nil {
		return fmt.Errorf("%s - failed to load bootstrap config: %w", logPrefix, err)
	}
	resolved := bootstrap.CreateResolvedBootstrap(bootstrapCfg)

	// Determine registry subject
	registrySubject := cfg.RegistrySubject
	if registrySubject == "" {
		registrySubject = resolved.GetSubject("system.registry")
	}
	if registrySubject == "" {
		registrySubject = commsutil.SubjectRegistry
	}
	slog.Info(fmt.Sprintf("%s - Registry subject: %s", logPrefix, registrySubject))

	// Step 2: Connect to NATS
	nc, err := commsutil.Connect(cfg.COMMSURL, cfg.COMMSName)
	if err != nil {
		return fmt.Errorf("%s - failed to connect to NATS: %w", logPrefix, err)
	}
	s.nc = nc
	slog.Info(fmt.Sprintf("%s - Connected to NATS at %s", logPrefix, cfg.COMMSURL))

	// Step 3: Connect to database
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		nc.Close()
		return fmt.Errorf("%s - failed to connect to database: %w", logPrefix, err)
	}
	s.pool = pool

	// Step 3b: Run migrations if enabled
	if cfg.RunMigrations {
		migrationSQL, err := db.LoadMigrationFiles(cfg.MigrationPath)
		if err != nil {
			pool.Close()
			nc.Close()
			return fmt.Errorf("%s - failed to load migrations: %w", logPrefix, err)
		}
		if err := db.RunMigrations(ctx, pool, migrationSQL); err != nil {
			pool.Close()
			nc.Close()
			return fmt.Errorf("%s - failed to run migrations: %w", logPrefix, err)
		}
		if err := db.SeedBootstrap(ctx, pool, cfg.BootstrapFile); err != nil {
			pool.Close()
			nc.Close()
			return fmt.Errorf("%s - failed to seed bootstrap capabilities: %w", logPrefix, err)
		}
	}

	// Step 4: Create registry (with NatsUrl for resolve responses)
	repo := db.NewRepository(pool)
	publisherOpts := &events.CommsPublisherOpts{}
	if cfg.ChangeEventSubject != "" {
		publisherOpts.GlobalChangeSubject = cfg.ChangeEventSubject
	}
	publisher := events.NewCommsPublisher(nc, publisherOpts)
	regConfig := registry.DefaultConfig()
	regConfig.NatsUrl = cfg.COMMSURL
	reg := registry.NewRegistry(registry.NewRegistryParams{
		Repo:      repo,
		Publisher: publisher,
		Config:    regConfig,
	})
	s.reg = reg

	// Step 6: Create dispatcher and subscribe
	disp := dispatcher.NewDispatcher(reg)

	requestTimeout := cfg.RequestTimeout
	sub, err := nc.Subscribe(registrySubject, func(msg *comms.Msg) {
		var req dispatcher.RegistryRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			slog.Error(fmt.Sprintf("%s - failed to decode request: %v", logPrefix, err))
			resp := &dispatcher.RegistryResponse{
				Ok: false,
				Error: &dispatcher.ErrorDetail{
					Code:    "INVALID_REQUEST",
					Message: "Failed to decode request",
				},
			}
			data, _ := json.Marshal(resp)
			msg.Respond(data)
			return
		}

		// Per-request context with timeout; optionally respect client deadline
		reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		if req.Ctx != nil && (req.Ctx.DeadlineMs > 0 || req.Ctx.TimeoutMs > 0) {
			ms := req.Ctx.DeadlineMs
			if ms <= 0 {
				ms = req.Ctx.TimeoutMs
			}
			if ms > 0 && time.Duration(ms)*time.Millisecond < requestTimeout {
				reqCtx, cancel = context.WithTimeout(ctx, time.Duration(ms)*time.Millisecond)
			}
		}
		defer cancel()

		// Dispatch
		resp := disp.Dispatch(reqCtx, &req)

		// Respond
		data, err := json.Marshal(resp)
		if err != nil {
			slog.Error(fmt.Sprintf("%s - failed to encode response: %v", logPrefix, err))
			return
		}
		msg.Respond(data)
	})
	if err != nil {
		pool.Close()
		nc.Close()
		return fmt.Errorf("%s - failed to subscribe to %s: %w", logPrefix, registrySubject, err)
	}
	slog.Info(fmt.Sprintf("%s - Subscribed to %s", logPrefix, registrySubject))

	// Step 5b: Subscribe to static bootstrap subject (returns bootstrap.json content).
	// Enriches bootstrap with natsUrl per capability and registry aliases from DB.
	bootstrapSub, err := nc.Subscribe(commsutil.SubjectBootstrap, func(msg *comms.Msg) {
		// Enrich capabilities with natsUrl (default NATS URL for system caps)
		for capRef, cap := range bootstrapCfg.Capabilities {
			if cap.NatsUrl == "" {
				cap.NatsUrl = cfg.COMMSURL
				bootstrapCfg.Capabilities[capRef] = cap
			}
		}

		// Load registry aliases from database for federation awareness
		if repo != nil {
			aliases, defaultAlias, err := reg.LoadRegistryAliases(ctx)
			if err == nil {
				registryAliases := make(map[string]bootstrap.BootstrapAliasEntry)
				for alias, natsUrl := range aliases {
					registryAliases[alias] = bootstrap.BootstrapAliasEntry{NatsUrl: natsUrl}
				}
				// Always include the default/local alias
				if _, ok := registryAliases[defaultAlias]; !ok {
					registryAliases[defaultAlias] = bootstrap.BootstrapAliasEntry{NatsUrl: cfg.COMMSURL}
				}
				bootstrapCfg.RegistryAliases = registryAliases
				bootstrapCfg.DefaultAlias = defaultAlias
			}
		}

		data, err := json.Marshal(bootstrapCfg)
		if err != nil {
			slog.Error(fmt.Sprintf("%s - bootstrap response encode: %v", logPrefix, err))
			return
		}
		msg.Respond(data)
	})
	if err != nil {
		sub.Unsubscribe()
		pool.Close()
		nc.Close()
		return fmt.Errorf("%s - failed to subscribe to %s: %w", logPrefix, commsutil.SubjectBootstrap, err)
	}
	defer bootstrapSub.Unsubscribe()
	slog.Info(fmt.Sprintf("%s - Subscribed to %s", logPrefix, commsutil.SubjectBootstrap))

	// Step 6: Start HTTP health server
	healthTimeout := cfg.HealthCheckTimeout
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleHome())
	mux.HandleFunc("/capability/", s.handleCapabilityDetail())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		healthCtx, cancel := context.WithTimeout(r.Context(), healthTimeout)
		defer cancel()
		h := reg.Health(healthCtx)
		w.Header().Set("Content-Type", "application/json")
		if h.Status != "healthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(h)
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	httpAddr := fmt.Sprintf(":%d", cfg.HTTPPort)
	s.httpServer = &http.Server{Addr: httpAddr, Handler: mux}
	go func() {
		slog.Info(fmt.Sprintf("%s - HTTP health server listening on %s", logPrefix, httpAddr))
		if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error(fmt.Sprintf("%s - HTTP server error: %v", logPrefix, err))
		}
	}()

	slog.Info(fmt.Sprintf("%s - Capabilities-server is ready", logPrefix))

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info(fmt.Sprintf("%s - Received signal %s, shutting down", logPrefix, sig))

	// Graceful shutdown
	sub.Unsubscribe()
	s.httpServer.Shutdown(ctx)
	reg.Close()
	nc.Drain()
	pool.Close()

	slog.Info(fmt.Sprintf("%s - Shutdown complete", logPrefix))
	return nil
}

// homePageTemplate is the HTML for the registry home page (white bg, black/blue text).
const homePageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Capabilities Registry</title>
  <style>
    * { box-sizing: border-box; }
    body { background: #fff; color: #000; font-family: system-ui, sans-serif; margin: 0; padding: 2rem; line-height: 1.5; }
    a { color: #0066cc; }
    h1, h2, h3 { color: #0066cc; }
    .status-healthy { color: #0066cc; font-weight: bold; }
    .status-unhealthy { color: #cc0000; font-weight: bold; }
    table { border-collapse: collapse; width: 100%; max-width: 900px; margin-top: 0.5rem; }
    th, td { text-align: left; padding: 0.5rem 0.75rem; border: 1px solid #ccc; }
    th { background: #f0f4f8; color: #0066cc; }
    .stat { font-weight: bold; color: #0066cc; }
    .meta { color: #333; font-size: 0.9rem; margin-top: 1rem; }
    section { margin-bottom: 2rem; }
    .error { color: #cc0000; }
  </style>
</head>
<body>
  <h1>Capabilities Registry</h1>
  <p class="meta">Registry health, statistics, and contents.</p>

  <section>
    <h2>Health</h2>
    <p>Status: <span class="status-{{.Health.Status}}">{{.Health.Status}}</span></p>
    <p>Database: {{if .Health.Checks.Database}}<span class="stat">OK</span>{{else}}<span class="error">Failed</span>{{end}}</p>
    <p>Timestamp: {{.Health.Timestamp}}</p>
  </section>

  <section>
    <h2>Statistics</h2>
    {{if .DiscoverError}}
    <p class="error">Could not load registry contents: {{.DiscoverError}}</p>
    {{else}}
    <p>Total capabilities: <span class="stat">{{.Discover.Pagination.Total}}</span></p>
    <p>Showing page {{.Discover.Pagination.Page}} of {{.Discover.Pagination.TotalPages}} ({{len .Discover.Capabilities}} on this page).</p>
    {{end}}
  </section>

  <section>
    <h2>Contents</h2>
    {{if .DiscoverError}}
    <p class="error">No contents available.</p>
    {{else}}
    {{if not .Discover.Capabilities}}
    <p>No capabilities registered.</p>
    {{else}}
    <table>
      <thead>
        <tr><th>Capability</th><th>App</th><th>Name</th><th>Default major</th><th>Latest version</th><th>Status</th></tr>
      </thead>
      <tbody>
        {{range .Discover.Capabilities}}
        <tr>
          <td><a href="/capability/{{.Cap}}">{{.Cap}}</a></td>
          <td>{{.App}}</td>
          <td>{{.Name}}</td>
          <td>{{.DefaultMajor}}</td>
          <td>{{.LatestVersion}}</td>
          <td>{{.Status}}</td>
        </tr>
        {{end}}
      </tbody>
    </table>
    {{end}}
    {{end}}
  </section>
</body>
</html>
`

// capabilityDetailPageTemplate is the HTML for a single capability detail page (describe output).
const capabilityDetailPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Describe.Cap}} – Capabilities Registry</title>
  <style>
    * { box-sizing: border-box; }
    body { background: #fff; color: #000; font-family: system-ui, sans-serif; margin: 0; padding: 2rem; line-height: 1.5; }
    a { color: #0066cc; }
    h1, h2, h3 { color: #0066cc; }
    table { border-collapse: collapse; width: 100%; max-width: 900px; margin-top: 0.5rem; }
    th, td { text-align: left; padding: 0.5rem 0.75rem; border: 1px solid #ccc; vertical-align: top; }
    th { background: #f0f4f8; color: #0066cc; width: 140px; }
    .meta { color: #333; font-size: 0.9rem; margin-top: 0.5rem; }
    section { margin-bottom: 2rem; }
    .error { color: #cc0000; }
    pre { background: #f5f5f5; padding: 0.75rem; overflow-x: auto; font-size: 0.85rem; margin: 0.25rem 0; border: 1px solid #eee; }
    .back { margin-bottom: 1rem; }
    .actions { margin: 1rem 0; }
    .btn { display: inline-block; padding: 0.5rem 1rem; background: #0066cc; color: #fff; text-decoration: none; border-radius: 4px; }
    .btn:hover { background: #0052a3; }
  </style>
</head>
<body>
  <p class="back"><a href="/">← Back to registry</a></p>
  {{if .DescribeError}}
  <p class="error">Could not load capability: {{.DescribeError}}</p>
  {{else}}
  <h1>{{.Describe.Cap}}</h1>
  {{if .Describe.Description}}<p class="meta">{{.Describe.Description}}</p>{{end}}
  <p class="actions"><a href="/capability/{{.Describe.Cap}}/docs" class="btn">View API (Swagger)</a></p>

  <section>
    <h2>Details</h2>
    <table>
      <tr><th>Capability</th><td>{{.Describe.Cap}}</td></tr>
      <tr><th>App</th><td>{{.Describe.App}}</td></tr>
      <tr><th>Name</th><td>{{.Describe.Name}}</td></tr>
      <tr><th>Version</th><td>{{.Describe.Version}}</td></tr>
      <tr><th>Major</th><td>{{.Describe.Major}}</td></tr>
      <tr><th>Status</th><td>{{.Describe.Status}}</td></tr>
      {{if .Describe.Tags}}
      <tr><th>Tags</th><td>{{range .Describe.Tags}}{{.}} {{end}}</td></tr>
      {{end}}
    </table>
  </section>

  {{if .Describe.Changelog}}
  <section>
    <h2>Changelog</h2>
    <pre>{{.Describe.Changelog}}</pre>
  </section>
  {{end}}

  <section>
    <h2>Methods</h2>
    {{if not .Describe.Methods}}
    <p>No methods defined.</p>
    {{else}}
    {{range .Describe.Methods}}
    <h3>{{.Name}}</h3>
    {{if .Description}}<p>{{.Description}}</p>{{end}}
    <p><strong>Modes:</strong> {{range .Modes}}{{.}} {{end}}</p>
    {{if .Tags}}<p><strong>Tags:</strong> {{range .Tags}}{{.}} {{end}}</p>{{end}}
    {{if or .InputSchema .OutputSchema}}
    <details>
      <summary>Schemas</summary>
      {{if .InputSchema}}<p><strong>Input:</strong></p><pre>{{json .InputSchema}}</pre>{{end}}
      {{if .OutputSchema}}<p><strong>Output:</strong></p><pre>{{json .OutputSchema}}</pre>{{end}}
    </details>
    {{end}}
    {{if .Examples}}
    <details>
      <summary>Examples</summary>
      {{range .Examples}}<pre>{{json .}}</pre>{{end}}
    </details>
    {{end}}
    {{end}}
    {{end}}
  </section>
  {{end}}
</body>
</html>
`

// homeData is the data passed to the home page template.
type homeData struct {
	Health        *registry.HealthOutput
	Discover      *registry.DiscoverOutput
	DiscoverError string
}

// handleHome returns an HTTP handler for the registry home page.
func (s *Server) handleHome() http.HandlerFunc {
	tmpl := template.Must(template.New("home").Parse(homePageTemplate))
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), s.cfg.HealthCheckTimeout)
		defer cancel()

		data := homeData{Health: s.reg.Health(ctx)}

		discover, err := s.reg.Discover(ctx, &registry.DiscoverInput{
			Status: "all",
			Limit:  100,
			Page:   1,
		})
		if err != nil {
			data.DiscoverError = err.Error()
		} else {
			data.Discover = discover
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			slog.Error(fmt.Sprintf("%s - home template execute: %v", logPrefix, err))
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	}
}

// capabilityDetailData is the data passed to the capability detail page template.
type capabilityDetailData struct {
	Describe      *registry.DescribeOutput
	DescribeError string
}

// openAPI3 types for generating specs from describe output.
type openAPI3Spec struct {
	OpenAPI string                    `json:"openapi"`
	Info    openAPI3Info              `json:"info"`
	Paths   map[string]openAPI3PathItem `json:"paths"`
}

type openAPI3Info struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

type openAPI3PathItem struct {
	Post *openAPI3Operation `json:"post,omitempty"`
}

type openAPI3Operation struct {
	Summary     string                 `json:"summary"`
	Description string                 `json:"description,omitempty"`
	OperationID string                 `json:"operationId"`
	RequestBody *openAPI3RequestBody   `json:"requestBody,omitempty"`
	Responses   map[string]openAPI3Response `json:"responses"`
}

type openAPI3RequestBody struct {
	Content map[string]openAPI3MediaType `json:"content"`
}

type openAPI3Response struct {
	Description string                 `json:"description"`
	Content     map[string]openAPI3MediaType `json:"content,omitempty"`
}

type openAPI3MediaType struct {
	Schema map[string]interface{} `json:"schema,omitempty"`
}

// buildOpenAPISpec builds an OpenAPI 3.0 spec from describe output (one path per method).
func buildOpenAPISpec(d *registry.DescribeOutput) *openAPI3Spec {
	paths := make(map[string]openAPI3PathItem)
	for _, m := range d.Methods {
		path := "/" + m.Name
		inputSchema := m.InputSchema
		if inputSchema == nil {
			inputSchema = map[string]interface{}{"type": "object"}
		}
		outputSchema := m.OutputSchema
		if outputSchema == nil {
			outputSchema = map[string]interface{}{"type": "object"}
		}
		paths[path] = openAPI3PathItem{
			Post: &openAPI3Operation{
				Summary:     m.Name,
				Description: m.Description,
				OperationID: m.Name,
				RequestBody: &openAPI3RequestBody{
					Content: map[string]openAPI3MediaType{
						"application/json": {Schema: inputSchema},
					},
				},
				Responses: map[string]openAPI3Response{
					"200": {
						Description: "Success",
						Content: map[string]openAPI3MediaType{
							"application/json": {Schema: outputSchema},
						},
					},
				},
			},
		}
	}
	desc := d.Description
	if desc == "" {
		desc = "Capability " + d.Cap
	}
	return &openAPI3Spec{
		OpenAPI: "3.0.0",
		Info: openAPI3Info{
			Title:       d.Cap,
			Description: desc,
			Version:     d.Version,
		},
		Paths: paths,
	}
}

// swaggerUIPage is the HTML that embeds Swagger UI from CDN and loads the OpenAPI spec.
const swaggerUIPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>API – {{.Cap}}</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      SwaggerUIBundle({
        url: "{{.SpecURL}}",
        dom_id: "#swagger-ui",
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ]
      });
    };
  </script>
</body>
</html>
`

// handleCapabilityDetail returns an HTTP handler for the capability detail page (describe), OpenAPI spec, and Swagger docs.
func (s *Server) handleCapabilityDetail() http.HandlerFunc {
	tmpl := template.Must(template.New("capabilityDetail").Funcs(template.FuncMap{
		"json": func(v interface{}) string {
			if v == nil {
				return ""
			}
			b, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return fmt.Sprintf("%v", v)
			}
			return string(b)
		},
	}).Parse(capabilityDetailPageTemplate))
	swaggerTmpl := template.Must(template.New("swagger").Parse(swaggerUIPage))
	return func(w http.ResponseWriter, r *http.Request) {
		pathCap := strings.TrimPrefix(r.URL.Path, "/capability/")
		if pathCap == "" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		capRef := pathCap
		suffix := ""
		if idx := strings.Index(pathCap, "/"); idx >= 0 {
			capRef = pathCap[:idx]
			suffix = pathCap[idx+1:]
		}
		capRef, err := url.PathUnescape(capRef)
		if err != nil {
			capRef = pathCap
			if idx := strings.Index(capRef, "/"); idx >= 0 {
				capRef = capRef[:idx]
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), s.cfg.HealthCheckTimeout)
		defer cancel()

		describe, err := s.reg.Describe(ctx, &registry.DescribeInput{Cap: capRef})
		if err != nil {
			if regErr, ok := err.(*registry.RegistryError); ok && regErr.Code == "NOT_FOUND" {
				http.NotFound(w, r)
				return
			}
			if suffix == "" {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				tmpl.Execute(w, capabilityDetailData{DescribeError: err.Error()})
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		switch suffix {
		case "openapi.json":
			spec := buildOpenAPISpec(describe)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "public, max-age=60")
			if err := json.NewEncoder(w).Encode(spec); err != nil {
				slog.Error(fmt.Sprintf("%s - openapi json encode: %v", logPrefix, err))
			}
			return
		case "docs":
			// Build absolute spec URL from request host so Swagger UI can fetch it
			specURL := "https://" + r.Host + "/capability/" + url.PathEscape(describe.Cap) + "/openapi.json"
			if r.TLS == nil {
				specURL = "http://" + r.Host + "/capability/" + url.PathEscape(describe.Cap) + "/openapi.json"
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			swaggerTmpl.Execute(w, map[string]string{"Cap": describe.Cap, "SpecURL": specURL})
			return
		case "":
			// fall through to detail page
		default:
			http.NotFound(w, r)
			return
		}

		data := capabilityDetailData{Describe: describe}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			slog.Error(fmt.Sprintf("%s - capability detail template execute: %v", logPrefix, err))
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	}
}
