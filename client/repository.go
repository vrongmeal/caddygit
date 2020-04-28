package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

// Defaults.
const (
	DefaultRemote = "origin"
	DefaultBranch = "master"
)

var errNoTag = errors.New("no tag found")

// RepositoryOpts are the options for creating a repository.
type RepositoryOpts struct {
	// URL (HTTP only) of the git repository.
	URL string `json:"url,omitempty"`

	// Path to clone the repository in. If path specified exists and is a git
	// repository, it simply opens the repo. If the path is not a repo and does
	// not exist, it creates a repo in that path.
	Path string `json:"path,omitempty"`

	// Branch (or tag) of the repository to clone. Defaults to `master` if
	// nothing is provided.
	Branch string `json:"branch,omitempty"`

	// Username and Password for authentication of private repositories.
	// If authenticating via access token, set the password equal to the value of
	// access token and username can be omitted.
	Username string `json:"auth_user,omitempty"`
	Password string `json:"auth_secret,omitempty"`

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
	branch         string
	fetchLatestTag bool
	refName        plumbing.ReferenceName
	auth           transport.AuthMethod
	singleBranch   bool
	depth          int
}

// NewRepository creates a new repository with given options.
func NewRepository(opts *RepositoryOpts) *Repository {
	r := &Repository{
		url:          opts.URL,
		path:         opts.Path,
		branch:       opts.Branch,
		singleBranch: opts.SingleBranch,
		depth:        opts.Depth,
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
func (r *Repository) Setup(ctx context.Context) error {
	var err error

	err = r.setRef(ctx)
	if err != nil {
		return err
	}

	// Try and open the repository first. If it opens successfully, we can
	// just configure the remote and continue.
	r.repo, err = git.PlainOpen(r.path)
	if err == nil {
		// If the repository already exists, set the remote to provided URL
		// Delete the old remote URL first and add the new remote
		err = r.repo.DeleteRemote(DefaultRemote)
		if err != nil && err != git.ErrRemoteNotFound {
			return err
		}

		_, err = r.repo.CreateRemote(&config.RemoteConfig{
			Name: DefaultRemote,
			URLs: []string{r.url},
		})
		if err != nil {
			return err
		}

		// Fetch and checkout to the given branch once so that pulls
		// are synchronous.
		err = r.fetch(ctx)
		if err != nil && err != git.NoErrAlreadyUpToDate {
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

	r.repo, err = git.PlainCloneContext(ctx, r.path, false, &git.CloneOptions{
		URL:           r.url,
		Auth:          r.auth,
		RemoteName:    DefaultRemote,
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

func (r *Repository) setRef(ctx context.Context) error {
	// First we fetch the references from remote and then compare it to
	// both the branch reference name and tag reference name. The reference
	// name that matches first is selected (preferably branch).
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: DefaultRemote,
		URLs: []string{r.url},
	})

	if err := remote.FetchContext(ctx, &git.FetchOptions{
		RemoteName: DefaultRemote,
		Depth:      r.depth,
		Auth:       r.auth,
		Tags:       git.AllTags,
	}); err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}

	refs, err := remote.List(&git.ListOptions{Auth: r.auth})
	if err != nil {
		return err
	}

	if r.branch == "" {
		r.refName = plumbing.NewBranchReferenceName(DefaultBranch)
	} else {
		branchRef := plumbing.NewBranchReferenceName(r.branch)
		tagRef := plumbing.NewTagReferenceName(r.branch)

		for _, ref := range refs {
			if ref.Name() == branchRef {
				r.refName = branchRef
				break
			}
			if ref.Name() == tagRef {
				r.refName = tagRef
				break
			}
		}

		if r.refName == plumbing.ReferenceName("") {
			return fmt.Errorf("reference with name '%s' not found", r.branch)
		}
	}

	return nil
}

// Update pulls/fetches updates from the remote repository into current worktree.
func (r *Repository) Update(ctx context.Context) error {
	if r.fetchLatestTag {
		lt, err := r.getLatestTag(ctx)
		if err != nil {
			return err
		}
		return r.checkout(lt)
	}

	if r.refName.IsBranch() {
		if err := r.pull(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (r *Repository) pull(ctx context.Context) error {
	wtree, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	if err := wtree.PullContext(ctx, &git.PullOptions{
		RemoteName:    DefaultRemote,
		ReferenceName: r.refName,
		SingleBranch:  r.singleBranch,
		Depth:         r.depth,
		Auth:          r.auth,
	}); err != nil {
		return err
	}

	return nil
}

func (r *Repository) fetch(ctx context.Context) error {
	if err := r.repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: DefaultRemote,
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

func (r *Repository) getLatestTag(ctx context.Context) (plumbing.ReferenceName, error) {
	nilReferenceName := plumbing.ReferenceName("")

	if err := r.fetch(ctx); err != nil {
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
