package git

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"go.uber.org/zap"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"

	"github.com/vrongmeal/caddygit"
	"github.com/vrongmeal/caddygit/module"
	"github.com/vrongmeal/caddygit/services/webhook"
)

func init() {
	caddy.RegisterModule(&Handler{})
}

// Handler implements the caddyhttp.MiddlewareHandler which can be used to
// create webhook handlers for git repo clients.
type Handler struct {
	Repository caddygit.RepositoryOpts `json:"repo,omitempty"`
	Commands   []caddygit.Command      `json:"commands_after,omitempty"`

	Secret  string          `json:"hook_secret,omitempty"`
	HookRaw json.RawMessage `json:"hook" caddy:"namespace=git.services.webhook inline_key=type"`

	client *module.Client
	whs    *webhook.Service
	log    *zap.Logger
	ctx    context.Context
	once   sync.Once
	setup  bool
}

// CaddyModule returns the module information.
func (*Handler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.git",
		New: func() caddy.Module { return new(Handler) },
	}
}

// Provision set's up h's configuration.
func (h *Handler) Provision(ctx caddy.Context) error {
	h.log = ctx.Logger(h)
	h.ctx = ctx.Context

	repl := caddy.NewReplacer()
	replaceableFields := []*string{
		&h.Secret,
	}
	for _, field := range replaceableFields {
		actual, err := repl.ReplaceOrErr(*field, false, true)
		if err != nil {
			return fmt.Errorf("error replacing fields: %v", err)
		}

		*field = actual
	}

	service := map[string]interface{}{
		"type":   "webhook",
		"secret": h.Secret,
		"hook":   h.HookRaw,
	}
	rawService, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("cannot marshal service JSON: %v", err)
	}

	h.client = &module.Client{
		RepositoryOpts: h.Repository,
		RawCommands:    h.Commands,
		ServiceRaw:     rawService,
	}

	err = h.client.Provision(ctx, h.log, repl)
	if err != nil {
		return fmt.Errorf("cannot provision client: %v", err)
	}

	return nil
}

// Validate ensures a's configuration is valid.
func (h *Handler) Validate() error {
	if err := h.client.Validate(); err != nil {
		return err
	}

	if _, ok := h.client.Service.(*webhook.Service); !ok {
		return fmt.Errorf("service not of type webhook; got %T", h.client.Service)
	}

	// Now that the config is validated repository can be setup.
	go func(handler *Handler) {
		if err := h.client.Setup(handler.ctx, handler.log); err != nil {
			handler.log.Error(
				"repository not setup",
				zap.Error(err),
				zap.String("path", handler.client.RepositoryOpts.Path),
			)
			return
		}
		// No mutex used since this is the only thread updating the field
		handler.setup = true
	}(h)

	return nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if !h.setup {
		return caddyhttp.Error(
			http.StatusNotFound,
			fmt.Errorf("page not found"),
		)
	}

	whs, ok := h.client.Service.(*webhook.Service)
	if !ok || whs == nil {
		return caddyhttp.Error(
			http.StatusInternalServerError,
			fmt.Errorf("webhook service <nil>"),
		)
	}

	if err := whs.ServeHTTP(w, r, next); err != nil {
		return err
	}

	go func(handler *Handler) {
		handler.log.Info("updating repository", zap.String("path", handler.client.RepositoryOpts.Path))

		if err := handler.client.Update(handler.ctx); err != nil {
			handler.log.Error(
				"cannot update repository",
				zap.Error(err),
				zap.String("path", handler.client.RepositoryOpts.Path))
			return
		}
	}(h)

	return nil
}

// Interface guards.
var (
	_ caddy.Module                = (*Handler)(nil)
	_ caddy.Provisioner           = (*Handler)(nil)
	_ caddy.Validator             = (*Handler)(nil)
	_ caddyhttp.MiddlewareHandler = (*Handler)(nil)
)
