// Package caddygit implements a Git Caddy module. This module can
// be used to deploy your website with a simple git push.
//
// This module starts a service that runs during the lifetime of the
// server. When the service starts, it clones the repository. While
// the server is still up, it pulls the latest every so often.
//
// Usage:
//     {
//         "apps": {
//             "git": {
//                 "repositories": [
//                     {
//                         "url": "https://github.com/caddyserver/caddy",
//                         "path": "/home/user/caddy"
//                     },
//                 ]
//     	       }
//         }
//     }
// The above example clones the caddy project into the specified path
// and the service pulls from the remote regularly (after 1 hour).
package caddygit

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/go-git/go-git/v5"
	"go.uber.org/zap"
)

const (
	defaultRemoteName = "origin"
	defaultBranchName = "master"
	defaultTagsMode   = git.AllTags

	latestTagKeyword = "{latest}"

	minServiceInterval = caddy.Duration(5 * time.Second)
	nilCaddyDuration   = caddy.Duration(0)
)

var (
	errBreakLoop = fmt.Errorf("break loop")
	log          *zap.Logger
)

func init() {
	if err := caddy.RegisterModule(Git{}); err != nil {
		caddy.Log().Fatal(err.Error())
	}
}

// Git is a module that makes it possible to deploy your site with a
// simple git push.
//
// This module starts a service that runs during the lifetime of the
// server. When the service starts, it clones the repository. While
// the server is still up, it pulls the latest every so often.
type Git struct {
	Repositories []Repository `json:"repositories"`

	wg *sync.WaitGroup
}

// CaddyModule returns the Caddy Git module information.
func (Git) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "git",
		New: func() caddy.Module {
			return &Git{wg: &sync.WaitGroup{}}
		},
	}
}

// Provision set's up g's configuration.
func (g *Git) Provision(c caddy.Context) error {
	log = c.Logger(g)
	return nil
}

// Validate ensures g's configuration is valid.
func (g *Git) Validate() error {
	return g.forEach(func(_ int, r *Repository) error {
		u, err := url.Parse(r.URL)
		if err != nil {
			return err
		}

		switch u.Scheme {
		case "http", "https":
		default:
			return fmt.Errorf(
				"not a valid url, requires http(s) url got %s",
				u.Scheme)
		}

		if r.Interval < minServiceInterval && r.Interval > nilCaddyDuration {
			return fmt.Errorf(
				"time interval should be greater than or equal to 5 seconds")
		}

		return nil
	})
}

// Start implements the caddy App interface. It begins the module
// execution by cloning the repository and starting the service.
func (g *Git) Start() error {
	return g.forEach(func(index int, repo *Repository) error {
		repo.index = index
		repo.then = newCommander(repo.Then)
		repo.thenLong = newCommander(repo.ThenLong)

		dur, err := repo.getInterval()
		if err != nil && err != errNoRunService {
			return fmt.Errorf(
				"cannot create service %d: %s",
				repo.index,
				err.Error())
		} else if err == nil {
			repo.service = newService(repo.runnerFunc, repo.errorFunc, dur)
		} else {
			repo.service = nil
		}

		g.wg.Add(1)
		go func(r *Repository) {
			defer g.wg.Done()

			err := r.clone()
			switch err {
			case nil:
			case git.ErrRepositoryAlreadyExists:
				if err = r.open(); err != nil {
					log.Error(
						"cannot open repository",
						zap.Error(err),
						zap.Int("repo", r.index))
					return
				}
			default:
				log.Error(
					"cannot clone repository",
					zap.Error(err),
					zap.Int("repo", r.index))
				return
			}

			// update repo once and then let the service handle.
			if err := r.runnerFunc(); err != nil {
				log.Error(
					"cannot update repository",
					zap.Error(err),
					zap.Int("repo", r.index))
			}

			if r.service != nil {
				r.service.start()
			}
		}(repo)

		return nil
	})
}

// Stop implements the caddy App interface. It quits the module
// execution by canceling the child contexts.
func (g *Git) Stop() error {
	if err := g.forEach(func(_ int, repo *Repository) error {
		if err := repo.then.quit(); err != nil {
			return err
		}

		if err := repo.thenLong.quit(); err != nil {
			return err
		}

		repo.service.quit()
		return nil
	}); err != nil {
		return err
	}

	g.wg.Wait()
	return nil
}

type iterFunc = func(int, *Repository) error

func (g *Git) forEach(f iterFunc) error {
	repos := g.Repositories
	for i := 0; i < len(repos); i++ {
		err := f(i, &repos[i])
		if err == nil {
			continue
		} else if err == errBreakLoop {
			break
		} else {
			return err
		}
	}
	return nil
}

// Interface guards
var (
	_ caddy.Module      = (*Git)(nil)
	_ caddy.Provisioner = (*Git)(nil)
	_ caddy.Validator   = (*Git)(nil)
	_ caddy.App         = (*Git)(nil)
)
