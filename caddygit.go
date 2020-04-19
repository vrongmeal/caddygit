// Package caddygit implements a Git Caddy module. This module can be used to deploy
// your website with a simple git push. This module starts a service that runs during
// the lifetime of the server. When the service starts, it clones the repository. While
// the server is still up, it pulls the latest every so often.
package caddygit

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/go-git/go-git/v5"
	"go.uber.org/zap"

	"github.com/vrongmeal/caddygit/commander"
	"github.com/vrongmeal/caddygit/repository"
	"github.com/vrongmeal/caddygit/service"
	"github.com/vrongmeal/caddygit/utils"
)

var (
	log *zap.Logger

	errNotGitDir = errors.New("given path is neither empty nor a git directory")
)

func init() {
	caddy.RegisterModule(App{})
}

// App is a module that makes it possible to deploy your site with a simple git push.
// This module starts a service that runs during the lifetime of the server. When the
// service starts, it clones the repository. While the server is still up, it pulls
// the latest every so often.
type App struct {
	Client []Client `json:"client,omitempty"`

	wg     *sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// Client contains the configuration for git client repository and service.
type Client struct {
	Repository repository.Opts `json:"repo,omitempty"`
	Service    service.Opts    `json:"service,omitempty"`
	Commands   commander.Opts  `json:"commands_after,omitempty"`

	r *repository.Repository
	s service.Service
	c *commander.Commander
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
	log = ctx.Logger(a)
	a.wg = &sync.WaitGroup{}
	a.ctx, a.cancel = context.WithCancel(ctx.Context)

	repl := caddy.NewReplacer()
	addGitVarsToReplacer(repl)

	for i := 0; i < len(a.Client); i++ {
		if err := a.provisionClient(&a.Client[i], repl); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) provisionClient(cl *Client, repl *caddy.Replacer) error {
	var err error

	ref, err := repl.ReplaceOrErr(cl.Repository.Branch, false, true)
	if err != nil {
		return err
	}

	cl.Repository.Branch = ref

	cl.s, err = service.New(a.ctx, &cl.Service)
	if err != nil {
		return err
	}

	cl.c = commander.New(a.ctx, cl.Commands)
	cl.c.OnStart = func(cmd commander.Command) {
		log.Info("running command", zap.String("cmd", cmd.String()))
	}
	cl.c.OnError = func(err error) {
		log.Error("failed to execute command", zap.Error(err))
	}

	cl.r = repository.New(a.ctx, &cl.Repository)

	return nil
}

// Validate ensures a's configuration is valid.
func (a *App) Validate() error {
	for i := 0; i < len(a.Client); i++ {
		if err := a.validateClient(&a.Client[i]); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) validateClient(cl *Client) error {
	if cl.Repository.URL == "" {
		return errors.New("cannot create repository with empty URL")
	}

	if cl.Repository.Path == "" {
		return errors.New("cannot create repository in empty path")
	}

	// We check if the path exists or not. If the path doesn't exist, it's validated OK
	// else we check if it's a git directory by opening it. If the directory doesn't open
	// successfully, it checks if the directory is empty. For non empty directory it
	// throws an error.
	dir, err := utils.IsDir(cl.Repository.Path)
	if err != nil && err != utils.ErrInvalidPath {
		return err
	} else if err == nil {
		if !dir {
			return errNotGitDir
		}

		_, err = git.PlainOpen(cl.Repository.Path)
		if err != nil {
			if err == git.ErrRepositoryNotExists {
				empty, err2 := utils.IsDirEmpty(cl.Repository.Path)
				if err2 != nil {
					return err
				}

				if !empty {
					return errNotGitDir
				}
			} else {
				return err
			}
		}
	}

	u, err := url.Parse(cl.Repository.URL)
	if err != nil {
		return err
	}

	switch u.Scheme {
	case "http", "https":
	default:
		return errors.New("given url scheme not supported: " + u.Scheme)
	}

	if cl.Service.Interval < caddy.Duration(5*time.Second) && cl.Service.Interval > 0 {
		return errors.New("time interval should be greater than or equal to 5 seconds")
	}

	return nil
}

// Start implements the caddy App interface. It begins the module execution by cloning
// or opening the repository and starting the service.
func (a *App) Start() error {
	log.Info("starting module")

	for i := 0; i < len(a.Client); i++ {
		a.wg.Add(1)
		go a.startClient(&a.Client[i])
	}

	return nil
}

func (a *App) startClient(cl *Client) {
	defer a.wg.Done()

	log.Info("setting up repository", zap.String("path", cl.Repository.Path))
	if err := cl.r.Setup(); err != nil {
		log.Error(
			"cannot setup repository",
			zap.Error(err),
			zap.String("path", cl.Repository.Path))
		return
	}

	log.Info("starting service", zap.String("path", cl.Repository.Path))
	if err := cl.s.Start(); err != nil {
		log.Error(
			"cannot start service",
			zap.Error(err),
			zap.String("path", cl.Repository.Path))
		return
	}

	for range cl.s.Tick() {
		log.Info("updating repository", zap.String("path", cl.Repository.Path))

		if err := cl.r.Update(); err != nil {
			log.Warn(
				"cannot update repository",
				zap.Error(err),
				zap.String("path", cl.Repository.Path))
			continue
		}

		if err := cl.c.Run(); err != nil {
			log.Warn(
				"cannot run commands",
				zap.Error(err),
				zap.String("path", cl.Repository.Path))
			continue
		}
	}
}

// Stop implements the caddy App interface. It quits the module execution.
func (a *App) Stop() error {
	a.cancel()
	a.wg.Wait()
	log.Info("stopped previous module")
	return nil
}

// Interface guards.
var (
	_ caddy.Module      = (*App)(nil)
	_ caddy.Provisioner = (*App)(nil)
	_ caddy.Validator   = (*App)(nil)
	_ caddy.App         = (*App)(nil)
)
