package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"

	"github.com/vrongmeal/caddygit"
)

func init() {
	caddy.RegisterModule(&Service{})
}

// Service ticks everytime a commit is pushed to the mentioned repository
// in the specified branch.
type Service struct {
	// Secret to verify the webhook is from correct source.
	Secret string `json:"secret,omitempty"`

	// Port to run the webhook server on. This server runs only when using the
	// service with the git plugin (app module). When using with the HTTP
	// module, this field is rendered useless.
	Port uint16 `json:"port,omitempty"`
	// Path tells the server which path to accept webhook requests.
	Path string `json:"path,omitempty"`

	// Hook specifies which webhook service is being used. All the hook types
	// are implemented as a different caddy module. All hooks can include
	// other configuration as well other than the common settings for all hooks.
	Hook    Webhook         `json:"-"`
	HookRaw json.RawMessage `json:"hook" caddy:"namespace=git.services.webhook inline_key=type"`

	repo    caddygit.RepositoryInfo
	addr    string
	handler http.Handler
	tick    chan error
}

// CaddyModule returns the Caddy module information.
func (*Service) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "git.services.webhook",
		New: func() caddy.Module { return new(Service) },
	}
}

// caddyNext to implement the http Handler interface.
type caddyNext struct{}

// ServeHTTP is an empty handler to implement caddy.Handler interface.
func (caddyNext) ServeHTTP(w http.ResponseWriter, r *http.Request) error { return nil }

// Provision sets s's configuration for the module.
func (s *Service) Provision(ctx caddy.Context) error {
	if s.HookRaw == nil {
		s.HookRaw = json.RawMessage(`{"type": "generic"}`)
	}

	hookIFace, err := ctx.LoadModule(s, "HookRaw")
	if err != nil {
		return fmt.Errorf("error loading module: %v", err)
	}

	var ok bool
	s.Hook, ok = hookIFace.(Webhook)
	if !ok {
		return fmt.Errorf("invalid hook configuration")
	}

	s.addr = net.JoinHostPort("0.0.0.0", fmt.Sprint(s.Port))
	s.tick = make(chan error, 1)
	return nil
}

// Validate validates s's configuration.
func (s *Service) Validate() error {
	if s.Path != "" && s.Path[0] != '/' {
		return fmt.Errorf("Path should be of the format `/path'")
	}

	return nil
}

// ConfigureRepo configures "s" with the repository information.
func (s *Service) ConfigureRepo(r caddygit.RepositoryInfo) error {
	s.repo = r
	return nil
}

// Start starts the webhook service and ticks for every favorable event.
func (s *Service) Start(ctx context.Context) <-chan error {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		s.tick <- s.ServeHTTP(w, r, caddyNext{})
	}

	if s.Path != "" {
		mux := http.NewServeMux()
		mux.HandleFunc(s.Path, func(w http.ResponseWriter, r *http.Request) {
			s.tick <- s.ServeHTTP(w, r, caddyNext{})
		})
		s.handler = mux
	} else {
		s.handler = http.HandlerFunc(handlerFunc)
	}

	go s.startService(ctx)

	return s.tick
}

// startService starts the webhook service.
func (s *Service) startService(ctx context.Context) {
	server := http.Server{
		Addr:    s.addr,
		Handler: s.handler,
	}

	errChan := make(chan error)
	go func(srv *http.Server, err chan<- error) {
		err <- srv.ListenAndServe()
	}(&server, errChan)

	select {
	case <-ctx.Done():
		// 15 seconds to shut down the server should be enough.
		sdCtx, sdCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer sdCancel()
		if err := server.Shutdown(sdCtx); err != nil {
			s.tick <- server.Close()
			close(s.tick)
			return
		}

		s.tick <- nil
		close(s.tick)
	case err := <-errChan:
		s.tick <- err
		close(s.tick)
	}
}

// ServeHTTP handles requests to the webhook payload URL.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	hc := HookConf{
		Secret:   s.Secret,
		RepoInfo: s.repo,
	}

	sc, err := s.Hook.Handle(r, &hc)
	if err != nil {
		w.WriteHeader(sc)
		return caddyhttp.Error(sc, err)
	}

	return next.ServeHTTP(w, r)
}

// Interface guard.
var (
	_ caddygit.Service            = (*Service)(nil)
	_ caddy.Module                = (*Service)(nil)
	_ caddy.Provisioner           = (*Service)(nil)
	_ caddy.Validator             = (*Service)(nil)
	_ caddyhttp.MiddlewareHandler = (*Service)(nil)
)
