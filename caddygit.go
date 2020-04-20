// Package caddygit implements a Git Caddy module. This module can be used to deploy
// your website with a simple git push. This module starts a service that runs during
// the lifetime of the server. When the service starts, it clones the repository. While
// the server is still up, it pulls the latest every so often.
package caddygit

import (
	"context"
	"fmt"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"go.uber.org/zap"

	"github.com/vrongmeal/caddygit/client"
)

func init() {
	caddy.RegisterModule(App{})
}

// App is a module that makes it possible to deploy your site with a simple git push.
// This module starts a service that runs during the lifetime of the server. When the
// service starts, it clones the repository. While the server is still up, it pulls
// the latest every so often.
type App struct {
	Clients []client.Client `json:"clients,omitempty"`

	wg     *sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	log    *zap.Logger
}

// CaddyModule returns the Caddy Git module information.
func (App) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "git",
		New: func() caddy.Module {
			return new(App)
		},
	}
}

// Provision set's up a's configuration.
func (a *App) Provision(ctx caddy.Context) error {
	a.wg = &sync.WaitGroup{}
	a.ctx, a.cancel = context.WithCancel(ctx.Context)
	a.log = ctx.Logger(a)

	repl := caddy.NewReplacer()

	if err := a.ProvisionClients(ctx, repl); err != nil {
		return err
	}

	return nil
}

// ProvisionClients sets up the clients configuration.
func (a *App) ProvisionClients(ctx caddy.Context, repl *caddy.Replacer) error {
	for i := 0; i < len(a.Clients); i++ {
		if err := a.Clients[i].Provision(ctx, a.log, repl); err != nil {
			return err
		}
	}

	return nil
}

// Validate ensures a's configuration is valid.
func (a *App) Validate() error {
	if err := a.ValidateClients(); err != nil {
		return err
	}

	return nil
}

// ValidateClients ensures that the clients have correct configuration.
func (a *App) ValidateClients() error {
	for i := 0; i < len(a.Clients); i++ {
		if err := a.Clients[i].Validate(); err != nil {
			return fmt.Errorf("client %d: %v", i, err)
		}
	}

	return nil
}

// Start implements the caddy App interface.
func (a *App) Start() error {
	a.log.Info("starting module")

	if err := a.StartClients(); err != nil {
		return err
	}

	return nil
}

// StartClients begins the module execution by cloning or opening the
// repositories and starting the services.
func (a *App) StartClients() error {
	for i := 0; i < len(a.Clients); i++ {
		a.wg.Add(1)
		go a.Clients[i].Start(a.ctx, a.wg, a.log)
	}

	return nil
}

// Stop implements the caddy App interface. It quits the module execution.
func (a *App) Stop() error {
	a.cancel()
	a.wg.Wait()
	a.log.Info("stopped previous module")
	return nil
}

// Interface guards.
var (
	_ caddy.Module      = (*App)(nil)
	_ caddy.Provisioner = (*App)(nil)
	_ caddy.Validator   = (*App)(nil)
	_ caddy.App         = (*App)(nil)
)
