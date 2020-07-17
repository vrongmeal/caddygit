package module

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/go-git/go-git/v5"
	"go.uber.org/zap"

	"github.com/vrongmeal/caddygit"
	"github.com/vrongmeal/caddygit/services/poll"
)

var (
	errInvalidPath = errors.New("filepath does not exist")
	errNotGitDir   = errors.New("given path is neither empty nor a git directory")
)

// Client contains the configuration for git client repository and service.
type Client struct {
	RepositoryOpts caddygit.RepositoryOpts `json:"repo,omitempty"`
	RawCommands    []caddygit.Command      `json:"commands_after,omitempty"`
	ServiceRaw     json.RawMessage         `json:"service,omitempty" caddy:"namespace=git.services inline_key=type"`

	Repo          *caddygit.Repository `json:"-"`
	CommandsAfter *caddygit.Commander  `json:"-"`
	Service       caddygit.Service     `json:"-"`
}

// Provision set's up cl's configuration.
func (c *Client) Provision(ctx caddy.Context, log *zap.Logger, repl *caddy.Replacer) error {
	// set the default service type to poll since it requires only one property,
	// i.e., interval, which can be easily be set by default
	if c.ServiceRaw == nil {
		c.ServiceRaw = json.RawMessage(`{"type": "poll"}`)
	}

	replaceableFields := []*string{
		&c.RepositoryOpts.Branch,
		&c.RepositoryOpts.Password,
		&c.RepositoryOpts.Path,
		&c.RepositoryOpts.URL,
		&c.RepositoryOpts.Username,
	}

	for _, field := range replaceableFields {
		actual, err := repl.ReplaceOrErr(*field, false, true)
		if err != nil {
			return fmt.Errorf("error replacing fields: %v", err)
		}

		*field = actual
	}

	serviceIface, err := ctx.LoadModule(c, "ServiceRaw")
	if err != nil {
		return fmt.Errorf("error loading module: %v", err)
	}

	var ok bool
	c.Service, ok = serviceIface.(caddygit.Service)
	if !ok {
		return fmt.Errorf("invalid service configuration")
	}

	if pollService, ok := c.Service.(*poll.Service); ok {
		if pollService.Interval == 0 {
			// set default interval equal to 1 hour
			pollService.Interval = caddy.Duration(1 * time.Hour)
		}
	}

	c.CommandsAfter = &caddygit.Commander{
		OnStart: func(cmd caddygit.Command) {
			log.Info("running command", zap.String("cmd", cmd.String()))
		},
		OnError: func(err error) {
			log.Warn("cannot run command", zap.Error(err))
		},
	}
	for i := range c.RawCommands {
		c.CommandsAfter.AddCommand(c.RawCommands[i])
	}

	if c.RepositoryOpts.Path == "" {
		// If the path is set empty for a repo, try to get the repo name from
		// the URL of the repo. If successful set it to "./<repo-name>" else
		// set it to current working directory, i.e., ".".
		var name string
		name, err = getRepoNameFromURL(c.RepositoryOpts.URL)
		if err != nil {
			c.RepositoryOpts.Path = "."
		} else {
			c.RepositoryOpts.Path = name
		}
	}

	// Get the absolute path (helpful while logging results)
	c.RepositoryOpts.Path, err = filepath.Abs(c.RepositoryOpts.Path)
	if err != nil {
		return fmt.Errorf("filepath.Abs(%#v): %v", c.RepositoryOpts.Path, err)
	}

	c.Repo = caddygit.NewRepository(&c.RepositoryOpts)

	return nil
}

// Validate ensures cl's configuration is valid.
func (c *Client) Validate() error {
	if c.RepositoryOpts.URL == "" {
		return fmt.Errorf("cannot create repository with empty URL")
	}

	if c.RepositoryOpts.Path == "" {
		return fmt.Errorf("cannot create repository in empty path")
	}

	// We check if the path exists or not. If the path doesn't exist, it's
	// validated OK else we check if it's a git directory by opening it. If the
	// directory doesn't open successfully, it checks if the directory is empty.
	// For non empty directory it throws an error.
	dir, err := isDir(c.RepositoryOpts.Path)
	if err != nil && err != errInvalidPath {
		return fmt.Errorf("error validating path: %v", err)
	} else if err == nil {
		if !dir {
			return errNotGitDir
		}

		_, err = git.PlainOpen(c.RepositoryOpts.Path)
		if err != nil {
			if err == git.ErrRepositoryNotExists {
				empty, err2 := isDirEmpty(c.RepositoryOpts.Path)
				if err2 != nil {
					return fmt.Errorf("error validating path: %v", err2)
				}

				if !empty {
					return errNotGitDir
				}
			} else {
				return fmt.Errorf("error validating path: %v", err)
			}
		}
	}

	u, err := url.Parse(c.RepositoryOpts.URL)
	if err != nil {
		return fmt.Errorf("invalid url: %v", err)
	}

	switch u.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("url scheme '%s' not supported", u.Scheme)
	}

	if pollService, ok := c.Service.(*poll.Service); ok {
		if pollService.Interval < caddy.Duration(5*time.Second) {
			return fmt.Errorf("interval for poll service cannot be less than 5 seconds")
		}
	}

	return nil
}

// Start begins the module execution by cloning or opening the repository
// and starting the service.
func (c *Client) Start(ctx context.Context, log *zap.Logger) error {
	log.Info("setting up repository", zap.String("path", c.RepositoryOpts.Path))
	if err := c.Repo.Setup(ctx); err != nil {
		return fmt.Errorf("cannot setup repository: %v", err)
	}

	// When the repo is setup for the first time, always run the commands_after
	// since they are most probably the setup commands for the repo which might
	// require building or starting a server.
	if err := c.CommandsAfter.Run(ctx); err != nil {
		return fmt.Errorf("cannot run commands: %v", err)
	}

	log.Info("starting service", zap.String("path", c.RepositoryOpts.Path))
	for serr := range c.Service.Start(ctx) {
		select {
		case <-ctx.Done():
			// For when update is received just before the context is canceled
			return ctx.Err()

		default:
			log.Info("updating repository", zap.String("path", c.RepositoryOpts.Path))
			if serr != nil {
				log.Error(
					"error updating the service",
					zap.Error(serr),
					zap.String("path", c.RepositoryOpts.Path))
				continue
			}

			if err := c.Repo.Update(ctx); err != nil {
				log.Warn(
					"cannot update repository",
					zap.Error(err),
					zap.String("path", c.RepositoryOpts.Path))
				continue
			}

			if err := c.CommandsAfter.Run(ctx); err != nil {
				log.Warn(
					"cannot run commands",
					zap.Error(err),
					zap.String("path", c.RepositoryOpts.Path))
				continue
			}
		}
	}

	return nil
}

// isDir tells if root is a directory.
func isDir(root string) (bool, error) {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, errInvalidPath
		}

		return false, err
	}

	if !info.IsDir() {
		return false, nil
	}

	return true, nil
}

// isDirEmpty tells if root directory is empty.
func isDirEmpty(root string) (bool, error) {
	f, err := os.Open(filepath.Clean(root))
	if err != nil {
		return false, err
	}
	defer f.Close() // nolint:errcheck

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}

	return false, err
}

// getRepoNameFromURL extracts the repo name from the HTTP URL of the repo.
func getRepoNameFromURL(u string) (string, error) {
	neturl, err := url.ParseRequestURI(u)
	if err != nil {
		return "", err
	}

	pathSegments := strings.Split(neturl.Path, "/")
	name := pathSegments[len(pathSegments)-1]
	return strings.TrimSuffix(name, ".git"), nil
}
