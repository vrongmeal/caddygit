package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/go-git/go-git/v5"
	"go.uber.org/zap"
)

var (
	errInvalidPath = errors.New("filepath does not exist")
	errNotGitDir   = errors.New("given path is neither empty nor a git directory")
)

// Client contains the configuration for git client repository and service.
type Client struct {
	RepositoryOpts RepositoryOpts  `json:"repo,omitempty"`
	RawCommands    []Command       `json:"commands_after,omitempty"`
	ServiceRaw     json.RawMessage `json:"service,omitempty" caddy:"namespace=git.services inline_key=type"`

	Repo          *Repository `json:"-"`
	CommandsAfter *Commander  `json:"-"`
	Service       Service     `json:"-"`
}

// Provision set's up cl's configuration.
func (cl *Client) Provision(ctx caddy.Context, log *zap.Logger, repl *caddy.Replacer) error {
	replaceableFields := []*string{
		&cl.RepositoryOpts.Branch,
		&cl.RepositoryOpts.Password,
		&cl.RepositoryOpts.Path,
		&cl.RepositoryOpts.URL,
		&cl.RepositoryOpts.Username,
	}

	for _, field := range replaceableFields {
		actual, err := repl.ReplaceOrErr(*field, false, true)
		if err != nil {
			return fmt.Errorf("error replacing fields: %v", err)
		}

		*field = actual
	}

	serviceIface, err := ctx.LoadModule(cl, "ServiceRaw")
	if err != nil {
		return fmt.Errorf("error loading module: %v", err)
	}

	var ok bool
	cl.Service, ok = serviceIface.(Service)
	if !ok {
		return fmt.Errorf("invalid service configuration")
	}

	cl.CommandsAfter = &Commander{
		Commands: cl.RawCommands,
		OnStart: func(cmd Command) {
			log.Info("running command", zap.String("cmd", cmd.String()))
		},
		OnError: func(err error) {
			log.Warn("cannot run command", zap.Error(err))
		},
	}

	cl.Repo = NewRepository(&cl.RepositoryOpts)

	return nil
}

// Validate ensures cl's configuration is valid.
func (cl *Client) Validate() error {
	if cl.RepositoryOpts.URL == "" {
		return fmt.Errorf("cannot create repository with empty URL")
	}

	if cl.RepositoryOpts.Path == "" {
		return fmt.Errorf("cannot create repository in empty path")
	}

	// We check if the path exists or not. If the path doesn't exist, it's validated OK
	// else we check if it's a git directory by opening it. If the directory doesn't open
	// successfully, it checks if the directory is empty. For non empty directory it
	// throws an error.
	dir, err := isDir(cl.RepositoryOpts.Path)
	if err != nil && err != errInvalidPath {
		return fmt.Errorf("error validating path: %v", err)
	} else if err == nil {
		if !dir {
			return errNotGitDir
		}

		_, err = git.PlainOpen(cl.RepositoryOpts.Path)
		if err != nil {
			if err == git.ErrRepositoryNotExists {
				empty, err2 := isDirEmpty(cl.RepositoryOpts.Path)
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

	u, err := url.Parse(cl.RepositoryOpts.URL)
	if err != nil {
		return fmt.Errorf("invalid url: %v", err)
	}

	switch u.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("url scheme '%s' not supported", u.Scheme)
	}

	pollService, ok := cl.Service.(*PollService)
	if ok {
		if pollService.Interval < caddy.Duration(5*time.Second) {
			return fmt.Errorf("interval for poll service cannot be less than 5 seconds")
		}
	}

	return nil
}

// Start begins the module execution by cloning or opening the repository and
// starting the service.
func (cl *Client) Start(ctx context.Context, wg *sync.WaitGroup, log *zap.Logger) {
	defer wg.Done()

	log.Info("setting up repository", zap.String("path", cl.RepositoryOpts.Path))
	if err := cl.Repo.Setup(ctx); err != nil {
		log.Error(
			"cannot setup repository",
			zap.Error(err),
			zap.String("path", cl.RepositoryOpts.Path))
		return
	}

	log.Info("starting service", zap.String("path", cl.RepositoryOpts.Path))
	for serr := range cl.Service.Start(ctx) {
		log.Info("updating repository", zap.String("path", cl.RepositoryOpts.Path))
		if serr != nil {
			log.Error(
				"error updating the service",
				zap.Error(serr),
				zap.String("path", cl.RepositoryOpts.Path))
			continue
		}

		if err := cl.Repo.Update(ctx); err != nil {
			log.Warn(
				"cannot update repository",
				zap.Error(err),
				zap.String("path", cl.RepositoryOpts.Path))
			continue
		}

		if err := cl.CommandsAfter.Run(ctx); err != nil {
			log.Warn(
				"cannot run commands",
				zap.Error(err),
				zap.String("path", cl.RepositoryOpts.Path))
			continue
		}
	}
}

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
