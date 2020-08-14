package github

import (
	"io/ioutil"
	"net/http"

	"github.com/caddyserver/caddy/v2"

	"github.com/vrongmeal/caddygit/services/webhook"
)

// Webhook implements a hook type which can be used to host the a project
// maintained on Github.
type Webhook struct{}

// CaddyModule returns the caddy module information.
func (Webhook) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "git.services.webhook.github",
		New: func() caddy.Module { return new(Webhook) },
	}
}

// Handle implements the webhook.Webhook interface.
func (Webhook) Handle(req *http.Request, hc *webhook.HookConf) (int, error) {
	if err := webhook.ValidateRequest(req); err != nil {
		return http.StatusBadRequest, err
	}

	_, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return http.StatusRequestTimeout, err
	}

	return http.StatusOK, nil
}

// Interface guards.
var (
	_ caddy.Module    = (*Webhook)(nil)
	_ webhook.Webhook = (*Webhook)(nil)
)
