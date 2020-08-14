package generic

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/vrongmeal/caddygit/services/webhook"
)

func init() {
	caddy.RegisterModule(Webhook{})
}

// reqBody is the structure of body expected in webhook request.
type reqBody struct {
	// Ref is the reference name of the repository.
	Ref string `json:"ref"`
}

// Webhook implements a hook type which can be used independent of platform
// used to host the git repository.
type Webhook struct{}

// CaddyModule returns the caddy module information.
func (Webhook) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "git.services.webhook.generic",
		New: func() caddy.Module { return new(Webhook) },
	}
}

// Handle implements the webhook.Webhook interface.
func (Webhook) Handle(req *http.Request, hc *webhook.HookConf) (int, error) {
	if err := webhook.ValidateRequest(req); err != nil {
		return http.StatusBadRequest, err
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return http.StatusRequestTimeout, err
	}

	var rBody reqBody

	err = json.Unmarshal(body, &rBody)
	if err != nil {
		return http.StatusBadRequest, err
	}

	refName := plumbing.ReferenceName(rBody.Ref)
	if refName.IsBranch() {
		if refName != hc.RepoInfo.ReferenceName {
			return http.StatusBadRequest, fmt.Errorf("event: push to branch %s", refName)
		}
	} else if refName.IsTag() {
		if !hc.RepoInfo.LatestTag && refName != hc.RepoInfo.ReferenceName {
			return http.StatusBadRequest, fmt.Errorf("event: push to tag %s", refName)
		}
	} else {
		// return error so the repo doesn't update
		return http.StatusBadRequest, fmt.Errorf("refName is neither a tag or a branch")
	}

	return http.StatusOK, nil
}

// Interface guards.
var (
	_ caddy.Module    = (*Webhook)(nil)
	_ webhook.Webhook = (*Webhook)(nil)
)
