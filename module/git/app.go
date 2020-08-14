package git

import (
	"context"
	"fmt"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"go.uber.org/zap"

	"github.com/vrongmeal/caddygit/module"
)

func init() {
	caddy.RegisterModule(&App{})
}

// App implements the module which can be used to create git clients.
type App struct {
	Clients []module.Client `json:"clients,omitempty"`

	logger *zap.Logger
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// CaddyModule returns the Caddy module information.
func (*App) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "git",
		New: func() caddy.Module { return new(App) },
	}
}

// Provision set's up a's configuration.
func (a *App) Provision(ctx caddy.Context) error {
	a.logger = ctx.Logger(a)
	a.ctx, a.cancel = context.WithCancel(ctx.Context)
	repl := caddy.NewReplacer()

	if err := a.provisionClients(ctx, repl); err != nil {
		return err
	}

	return nil
}

// Validate ensures a's configuration is valid.
func (a *App) Validate() error {
	if err := a.validateClients(); err != nil {
		return err
	}

	return nil
}

// Start implements the caddy App interface. It executes the module.
func (a *App) Start() error {
	a.logger.Info("starting module")

	if err := a.startClients(); err != nil {
		return err
	}

	return nil
}

// startClients begins the module execution by cloning or opening the
// repositories and starting the services.
func (a *App) startClients() error {
	for i := 0; i < len(a.Clients); i++ {
		a.wg.Add(1)
		go func(idx int, ctx context.Context, log *zap.Logger) {
			defer a.wg.Done()

			if err := a.Clients[idx].Start(ctx, log); err != nil {
				log.Error(newClientErr(idx, err).Error())
			}
		}(i, a.ctx, a.logger)
	}

	return nil
}

// Stop implements the caddy App interface. It quits the module execution.
func (a *App) Stop() error {
	a.cancel()
	a.wg.Wait()
	a.logger.Info("stopped previous module")
	return nil
}

// provisionClients sets up the clients configuration.
func (a *App) provisionClients(ctx caddy.Context, repl *caddy.Replacer) error {
	for i := 0; i < len(a.Clients); i++ {
		if err := a.Clients[i].Provision(ctx, a.logger, repl); err != nil {
			return newClientErr(i, err)
		}
	}

	return nil
}

// validateClients ensures that the clients have correct configuration.
func (a *App) validateClients() error {
	for i := 0; i < len(a.Clients); i++ {
		if err := a.Clients[i].Validate(); err != nil {
			return newClientErr(i, err)
		}
	}

	return nil
}

// newIterErr returns an error occurred in between an iteration.
func newIterErr(prefix string, idx int, err error) error {
	return fmt.Errorf("%s %d: %v", prefix, idx, err)
}

// newClientErr creates an iter error for idx'th client.
func newClientErr(idx int, err error) error {
	return newIterErr("client", idx, err)
}

// Interface guards.
var (
	_ caddy.Module      = (*App)(nil)
	_ caddy.Provisioner = (*App)(nil)
	_ caddy.Validator   = (*App)(nil)
	_ caddy.App         = (*App)(nil)
)
