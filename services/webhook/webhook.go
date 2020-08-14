package webhook

import (
	"fmt"
	"net/http"

	"github.com/vrongmeal/caddygit"
)

// HookConf is the configuration of hook that can be used by different
// webhook services to handle events.
type HookConf struct {
	// Secret used to authorize the origin of webhook request.
	Secret string

	// RepoInfo is the repository information.
	RepoInfo caddygit.RepositoryInfo
}

// Webhook is anything that handles a POST request with events and if the
// desired event occurs, causes Handle() to return nil error.
type Webhook interface {
	// Handle when a favorable event occurs. Returns status code and error.
	Handle(*http.Request, *HookConf) (int, error)
}

// ValidateRequest validates webhook request for common parameters such as
// method type should be POST.
//
// This is a helper function and is not enforced on the `ServeHTTP` method
// for all the webhooks, rather implemented exclusively for each hook type.
func ValidateRequest(r *http.Request) error {
	if r.Method != http.MethodPost {
		return fmt.Errorf("only %s method accepted; got %s", http.MethodPost, r.Method)
	}

	return nil
}
