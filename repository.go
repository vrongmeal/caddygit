package caddygit

import (
	"errors"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"go.uber.org/zap"
)

const (
	nilReferenceName = plumbing.ReferenceName("")
)

var (
	errNoTag        = errors.New("no tag found")
	errNoRunService = errors.New("service cannot run for negative interval")
)

// Repository represents a git repository managed by the Caddy Git
// module. It fetches and updates the repository at regular
// intervals and executes the required actions.
type Repository struct {
	// URL (HTTP) of the git repository.
	URL string `json:"url"`

	// Path to clone the repository in.
	Path string `json:"path"`

	// Tag of the repository to checkout to. Can use `{latest}` as a
	// placeholder to fetch the latest tag. If nothing is provided,
	// the reference is taken from the branch.
	Tag string `json:"tag,omitempty"`

	// Branch of the repository to clone. Defaults to master if nothing
	// is provided. If `tag` is set to `{latest}` then this branch is
	// taken as the base for finding the latest tag.
	Branch string `json:"branch,omitempty"`

	// Username and Password for authentication of private repositories.
	// If authenticating via access token, set the password equal to the
	// value of access token and username can be omitted.
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`

	// SingleBranch specifies whether to clone only the specified branch.
	SingleBranch bool `json:"single_branch,omitempty"`

	// Depth of commits to fetch.
	Depth int `json:"depth,omitempty"`

	// Interval is the number of seconds between pulls. Defaults to 1 hour.
	// Minimum interval is 5 seconds. An negative interval value disables
	// periodic pull.
	Interval caddy.Duration `json:"interval,omitempty"`

	// Then is the list of commands to execute after successful update
	// of the repository. Commands specified here block the service.
	Then []string `json:"then,omitempty"`

	// ThenLong is the list of commands that run in background.
	// These commands do not block the service and are executed after
	// the "then" commands.
	ThenLong []string `json:"then_long,omitempty"`

	index          int
	repo           *git.Repository
	service        *service
	then, thenLong *commander
}

// isRefLatestTag tells if the repository has latest tag configuration.
func (r *Repository) isRefLatestTag() bool {
	return r.Tag == latestTagKeyword
}

// isRefBranch tells if the reference is branch or not.
func (r *Repository) isRefBranch() bool {
	return r.Tag == ""
}

// isRefTag tells if the reference is tag or not. It also returns true if
// the tag is specified to be latest.
func (r *Repository) isRefTag() bool {
	return r.Tag != ""
}

// auth returns the authentication method for the git repository.
func (r *Repository) auth() transport.AuthMethod {
	if r.Username == "" && r.Password == "" {
		return nil
	}

	username := r.Username
	if r.Username == "" {
		// Username cannot be an empty string if access token
		// is used for authorization.
		username = "caddy"
	}

	return &http.BasicAuth{
		Username: username,
		Password: r.Password,
	}
}

// branchReferenceName gets the reference name from the branch of
// the git repository.
func (r *Repository) branchReferenceName() plumbing.ReferenceName {
	branch := r.Branch
	if branch == "" {
		branch = defaultBranchName
	}

	return plumbing.NewBranchReferenceName(branch)
}

// referenceName gets the reference name of the repo to checkout to.
func (r *Repository) referenceName() plumbing.ReferenceName {
	if r.isRefBranch() || r.isRefLatestTag() {
		return r.branchReferenceName()
	}

	return plumbing.NewTagReferenceName(r.Tag)
}

// cloneOptions return the clone options for the git repository.
func (r *Repository) cloneOptions() *git.CloneOptions {
	return &git.CloneOptions{
		URL:           r.URL,
		Auth:          r.auth(),
		RemoteName:    defaultRemoteName,
		ReferenceName: r.referenceName(),
		SingleBranch:  r.SingleBranch,
		Depth:         r.Depth,
		Tags:          defaultTagsMode,
	}
}

// fetchOptions return the fetch options for the git repository.
func (r *Repository) fetchOptions() *git.FetchOptions {
	return &git.FetchOptions{
		RemoteName: defaultRemoteName,
		Depth:      r.Depth,
		Auth:       r.auth(),
		Tags:       defaultTagsMode,
	}
}

// pullOptions returns the pull options for the git repository.
func (r *Repository) pullOptions() *git.PullOptions {
	return &git.PullOptions{
		RemoteName:    defaultRemoteName,
		ReferenceName: r.referenceName(),
		SingleBranch:  r.SingleBranch,
		Depth:         r.Depth,
		Auth:          r.auth(),
	}
}

// checkoutOptions returns the checkout options for the git repository.
// If ref is equal to `nilReferenceName`, the reference is taken as the
// default reference by the method `referenceName`.
func (r *Repository) checkoutOptions(
	ref plumbing.ReferenceName) *git.CheckoutOptions {
	return &git.CheckoutOptions{Branch: ref}
}

// clone clones the repository in the given path and checks-out
// to the required reference.
func (r *Repository) clone() error {
	log.Info(
		"cloning repository",
		zap.Int("repo", r.index),
		zap.String("url", r.URL),
		zap.String("path", r.Path))

	repo, err := git.PlainClone(
		r.Path, false, r.cloneOptions())
	if err != nil {
		return err
	}

	r.repo = repo
	return nil
}

// open opens the repository in the given path.
func (r *Repository) open() error {
	log.Info(
		"opening existing repository",
		zap.Int("repo", r.index),
		zap.String("path", r.Path))

	repo, err := git.PlainOpen(r.Path)
	if err != nil {
		return err
	}

	r.repo = repo
	return nil
}

// pull pulls the latest changes from the remote repository.
func (r *Repository) pull() error {
	wtree, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	if err := wtree.Pull(r.pullOptions()); err != nil {
		if err == git.NoErrAlreadyUpToDate {
			return nil
		}

		return err
	}

	return nil
}

// fetch fetches the details of changes from the remote to the
// local repository.
func (r *Repository) fetch() error {
	if err := r.repo.Fetch(r.fetchOptions()); err != nil {
		if err == git.NoErrAlreadyUpToDate {
			return nil
		}

		return err
	}

	return nil
}

// checkout checksout to the given reference name.
func (r *Repository) checkout(ref plumbing.ReferenceName) error {
	wtree, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	if err := wtree.Checkout(r.checkoutOptions(ref)); err != nil {
		return err
	}

	return nil
}

// getLatestTag gets the latest tag for the repo from the remote.
// Returns `errNoNewTag` when new tag is not found.
func (r *Repository) getLatestTag() (plumbing.ReferenceName, error) {
	if err := r.fetch(); err != nil {
		return nilReferenceName, err
	}

	if err := r.checkout(r.branchReferenceName()); err != nil {
		return nilReferenceName, err
	}

	tagsMap := make(map[plumbing.Hash]*plumbing.Reference)

	tags, err := r.repo.Tags()
	if err != nil {
		return nilReferenceName, err
	}

	err = tags.ForEach(func(ref *plumbing.Reference) error {
		tagsMap[ref.Hash()] = ref
		return nil
	})
	if err != nil {
		return nilReferenceName, err
	}

	head, err := r.repo.Head()
	if err != nil {
		return nilReferenceName, err
	}

	commits, err := r.repo.Log(&git.LogOptions{
		From:  head.Hash(),
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		return nilReferenceName, err
	}

	var tag *plumbing.Reference
	if err := commits.ForEach(func(commit *object.Commit) error {
		if t, ok := tagsMap[commit.Hash]; ok {
			tag = t
		}
		if tag != nil {
			return storer.ErrStop
		}
		return nil
	}); err != nil {
		return nilReferenceName, err
	}

	if tag == nil {
		return nilReferenceName, errNoTag
	}

	return tag.Name(), nil
}

// setWTree either checks-out to the latest tag or pulls latest changes
// from the remote.
func (r *Repository) setWTree() error {
	// If the tag is specified to be "{latest}", get the latest tag
	// and checkout to it.
	if r.isRefLatestTag() {
		tag, err := r.getLatestTag()
		switch err {
		case nil:
			return r.checkout(tag)
		case errNoTag:
			// do nothing
		default:
			return err
		}
	} else if r.isRefBranch() { // If the ref is branch instead of tag.
		if err := r.pull(); err != nil {
			return err
		}
	} // else do nothing as given reference is a specific tag.

	return nil
}

// getInterval returns the interval after which the service should be run.
func (r *Repository) getInterval() (time.Duration, error) {
	if r.isRefTag() && !r.isRefLatestTag() {
		return -1, errNoRunService
	}

	if r.Interval < nilCaddyDuration {
		return -1, errNoRunService
	}

	if r.Interval == nilCaddyDuration {
		// Default interval for service is 1 hour
		return time.Hour, nil
	}

	return time.Duration(r.Interval), nil
}

// runnerFunc is the function to be run for setting up the repository.
func (r *Repository) runnerFunc() error {
	log.Info(
		"updating repository",
		zap.Int("repo", r.index),
		zap.String("url", r.URL),
		zap.String("path", r.Path))
	if err := r.setWTree(); err != nil {
		return err
	}

	if err := r.then.run(); err != nil {
		return err
	}

	go func(cmds *commander) {
		if err := cmds.run(); err != nil {
			r.errorFunc(err)
			return
		}
	}(r.thenLong)

	return nil
}

// errorFunc is the function which is run when service encounters error.
func (r *Repository) errorFunc(err error) {
	log.Error(
		"cannot update repository",
		zap.Error(err),
		zap.Int("repo", r.index))
}
