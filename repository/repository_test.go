package repository

import (
	"context"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/vrongmeal/caddygit/utils"
)

func TestRepository_New(t *testing.T) { // nolint:gocyclo
	var repo *Repository
	var ctx context.Context
	opts := &Opts{
		URL:  "https://github.com/abc/def",
		Path: "/to/my/repo",
	}

	repo = New(ctx, opts)
	if repo.url != opts.URL {
		t.Error("Repo URLs don't match")
	}
	if repo.path != opts.Path {
		t.Error("Repo paths don't match")
	}
	if repo.remoteName != DefaultRemote {
		t.Error("Repo remote name is not set to default")
	}
	if repo.fetchLatestTag {
		t.Error("Repo should not fetch latest tag if branch empty")
	}
	if repo.refName != plumbing.NewBranchReferenceName(DefaultBranch) {
		t.Error("Repo reference is not set to default")
	}
	if repo.auth != nil {
		t.Error("Auhentication should be nil for no username and password")
	}
	if repo.singleBranch {
		t.Error("Repo should not fetch single branch if not specified")
	}
	if repo.depth != 0 {
		t.Error("Repo should not fetch limited commits if not specified")
	}

	opts.Remote = "upstream"
	opts.Branch = "develop"
	opts.Password = "password"
	opts.SingleBranch = true
	opts.Depth = 2
	repo = New(ctx, opts)
	if repo.remoteName != opts.Remote {
		t.Errorf("Provided remote '%s', got remote '%s'",
			opts.Remote, repo.remoteName)
	}
	if repo.fetchLatestTag {
		t.Error("Repo should not fetch latest tag for given branch name")
	}
	if repo.refName != plumbing.NewBranchReferenceName(opts.Branch) {
		t.Errorf("Expected reference name to be '%v', got '%v'",
			plumbing.NewBranchReferenceName(opts.Branch),
			repo.refName)
	}
	if repo.auth == nil {
		t.Error("Authentication should not be nil for given password")
	}
	httpAuth, ok := repo.auth.(*http.BasicAuth)
	if !ok {
		t.Error("Authentication should be basic HTTP type")
	}
	if httpAuth.Username == "" {
		t.Error("Username should not be empty fot non-nil auth")
	}
	if httpAuth.Password != opts.Password {
		t.Errorf("Provided password '%s', got password '%s'",
			opts.Password, httpAuth.Password)
	}
	if !repo.singleBranch {
		t.Errorf("Provided single branch '%t', got single branch '%t'",
			opts.SingleBranch, repo.singleBranch)
	}
	if repo.depth != opts.Depth {
		t.Errorf("Provided depth '%d', got depth '%d'",
			opts.Depth, repo.depth)
	}

	tagName := "v1.2.3"
	opts.Branch = utils.NewTagReferenceName(tagName).String()
	opts.Username = "username"
	repo = New(ctx, opts)
	if repo.fetchLatestTag {
		t.Error("Repo should not fetch latest tag for given tag")
	}
	if repo.refName != plumbing.NewTagReferenceName(tagName) {
		t.Errorf("Expected reference name to be '%v', got '%v'",
			plumbing.NewTagReferenceName(tagName),
			repo.refName)
	}
	if repo.auth == nil {
		t.Error("Authentication should not be nil for given password")
	}
	httpAuth, ok = repo.auth.(*http.BasicAuth)
	if !ok {
		t.Error("Authentication should be basic HTTP type")
	}
	if httpAuth.Username != opts.Username {
		t.Errorf("Provided username '%s', got username '%s'",
			opts.Username, httpAuth.Username)
	}
	if httpAuth.Password != opts.Password {
		t.Errorf("Provided password '%s', got password '%s'",
			opts.Password, httpAuth.Password)
	}

	branchName := "upstream"

	opts.Branch = utils.NewLatestCommitReferenceName(branchName).String()
	repo = New(ctx, opts)
	if repo.fetchLatestTag {
		t.Error("Repo should not fetch latest tag for given latest commit ref")
	}
	if repo.refName != plumbing.NewBranchReferenceName(branchName) {
		t.Errorf("Expected reference name to be '%v', got '%v'",
			plumbing.NewBranchReferenceName(branchName),
			repo.refName)
	}

	opts.Branch = utils.NewLatestTagReferenceName(branchName).String()
	repo = New(ctx, opts)
	if !repo.fetchLatestTag {
		t.Error("Repo should not fetch latest tag for latest tag ref")
	}
	if repo.refName != plumbing.NewBranchReferenceName(branchName) {
		t.Errorf("Expected reference name to be '%v', got '%v'",
			plumbing.NewBranchReferenceName(branchName),
			repo.refName)
	}
}
