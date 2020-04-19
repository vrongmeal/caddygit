// Package repository defines a git repository.
package repository

import (
	"context"
	"errors"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/vrongmeal/caddygit/utils"
)

// Defaults.
const (
	DefaultRemote = "origin"
	DefaultBranch = "master"
)

var errNoTag = errors.New("no tag found")

// Opts are the options for creating a repository.
type Opts struct {
	// URL (HTTP only) of the git repository.
	URL string `json:"url,omitempty"`

	// Path to clone the repository in. If path specified exists and is a git
	// repository, it simply opens the repo. If the path is not a repo and does
	// not exist, it creates a repo in that path.
	Path string `json:"path,omitempty"`

	// Remote name. Defaults to "origin".
	Remote string `json:"remote,omitempty"`

	// Branch (or tag) of the repository to clone. Defaults to `master` if nothing is provided.
	// Can be set using placeholders:
	//  `{git.ref.branch.<branch>}` for branch name. Equivalent to `<branch>`.
	//  `{git.ref.branch.<branch>.latest_commit}` is same as above.
	//  `{git.ref.latest_commit}` is same as above for default branch. Equivalent to empty string.
	//  `{git.ref.branch.<branch>.latest_tag}` fetches latest tag for given branch.
	//  `{git.ref.latest_tag}` is same as above for default branch.
	//  `{git.ref.tag.<tag>}` for tag name.
	Branch string `json:"branch,omitempty"`

	// Username and Password for authentication of private repositories.
	// If authenticating via access token, set the password equal to the value of
	// access token and username can be omitted.
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`

	// SingleBranch specifies whether to clone only the specified branch.
	SingleBranch bool `json:"single_branch,omitempty"`

	// Depth of commits to fetch.
	Depth int `json:"depth,omitempty"`
}

// Repository is a git repository in the given `Path` with `Remote` URL
// equal to `URL`.
type Repository struct {
	repo *git.Repository

	url            string
	path           string
	remoteName     string
	fetchLatestTag bool
	refName        plumbing.ReferenceName
	auth           transport.AuthMethod
	singleBranch   bool
	depth          int

	ctx context.Context
}

// New creates a new repository with given options.
func New(ctx context.Context, opts *Opts) *Repository {
	r := &Repository{
		url:          opts.URL,
		path:         opts.Path,
		singleBranch: opts.SingleBranch,
		depth:        opts.Depth,
		ctx:          ctx,
	}

	r.remoteName = DefaultRemote
	if opts.Remote != "" {
		r.remoteName = opts.Remote
	}

	ref := utils.ReferenceName(opts.Branch)
	if !ref.IsValid() {
		branch := DefaultBranch
		if opts.Branch != "" {
			branch = opts.Branch
		}
		r.refName = plumbing.NewBranchReferenceName(branch)
	} else {
		r.fetchLatestTag = ref.IsLatestTag()
		if ref.IsTag() {
			r.refName = plumbing.NewTagReferenceName(ref.Name())
		} else { // whether latest commit or latest tag
			branch := DefaultBranch
			if ref.Name() != "" {
				branch = ref.Name()
			}
			r.refName = plumbing.NewBranchReferenceName(branch)
		}
	}

	if opts.Username == "" && opts.Password == "" {
		r.auth = nil
	} else {
		username := "caddy"
		if opts.Username != "" {
			username = opts.Username
		}

		r.auth = &http.BasicAuth{
			Username: username,
			Password: opts.Password,
		}
	}

	return r
}

// Setup initializes the git repository by either cloning or opening it.
func (r *Repository) Setup() error {
	var err error

	// Try and open the repository first. If it opens successfully, we can
	// just configure the remote and continue.
	r.repo, err = git.PlainOpen(r.path)
	if err == nil {
		// If the repository already exists, set the remote to provided URL
		// Delete the old remote URL first and add the new remote
		err = r.repo.DeleteRemote(r.remoteName)
		if err != nil && err != git.ErrRemoteNotFound {
			return err
		}

		_, err = r.repo.CreateRemote(&config.RemoteConfig{
			Name: r.remoteName,
			URLs: []string{r.url},
		})
		if err != nil {
			return err
		}

		// Fetch and checkout to the given branch once so that pulls
		// are synchronous.
		err = r.fetch()
		if err != nil {
			return err
		}

		err = r.checkout(r.refName)
		if err != nil {
			return err
		}

		return nil
	} else if err != git.ErrRepositoryNotExists {
		return err
	}

	r.repo, err = git.PlainCloneContext(r.ctx, r.path, false, &git.CloneOptions{
		URL:           r.url,
		Auth:          r.auth,
		RemoteName:    r.remoteName,
		ReferenceName: r.refName,
		SingleBranch:  r.singleBranch,
		Depth:         r.depth,
		Tags:          git.AllTags,
	})
	if err != nil {
		return err
	}

	return nil
}

// Update pulls/fetches updates from the remote repository into current worktree.
func (r *Repository) Update() error {
	// If the tag is specified to be "{latest}", get the latest tag
	// and checkout to it.
	if r.fetchLatestTag {
		tag, err := r.getLatestTag()
		switch err {
		case nil:
			return r.checkout(tag)
		case errNoTag:
			// do nothing
		default:
			return err
		}
	} else if r.refName.IsBranch() { // If the ref is branch instead of tag.
		if err := r.pull(); err != nil {
			return err
		}
	} // else do nothing as given reference is a specific tag.

	return nil
}

func (r *Repository) pull() error {
	wtree, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	if err := wtree.PullContext(r.ctx, &git.PullOptions{
		RemoteName:    r.remoteName,
		ReferenceName: r.refName,
		SingleBranch:  r.singleBranch,
		Depth:         r.depth,
		Auth:          r.auth,
	}); err != nil {
		return err
	}

	return nil
}

func (r *Repository) fetch() error {
	if err := r.repo.FetchContext(r.ctx, &git.FetchOptions{
		RemoteName: r.remoteName,
		Depth:      r.depth,
		Auth:       r.auth,
		Tags:       git.AllTags,
	}); err != nil {
		return err
	}

	return nil
}

func (r *Repository) checkout(ref plumbing.ReferenceName) error {
	wtree, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	if err := wtree.Checkout(&git.CheckoutOptions{Branch: ref}); err != nil {
		return err
	}

	return nil
}

func (r *Repository) getLatestTag() (plumbing.ReferenceName, error) {
	nilReferenceName := plumbing.ReferenceName("")

	if err := r.fetch(); err != nil {
		return nilReferenceName, err
	}

	if err := r.checkout(r.refName); err != nil {
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
