package utils

import (
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

const separateDelimeter = "|"

// ReferenceName is the caddygit's way of representation of references.
type ReferenceName string

// referenceType is the type of references.
type referenceType string

// Types of references.
const (
	latestCommit referenceType = "latest_commit"
	latestTag    referenceType = "latest_tag"
	tag          referenceType = "tag"
)

// newReferenceName creates a reference name for given ReferenceOpts.
func newReferenceName(name string, typ referenceType) ReferenceName {
	return ReferenceName(string(typ) + separateDelimeter + name)
}

// NewLatestCommitReferenceName creates a reference name for given ReferenceOpts.
func NewLatestCommitReferenceName(name string) ReferenceName {
	return newReferenceName(name, latestCommit)
}

// NewLatestTagReferenceName creates a reference name for given ReferenceOpts.
func NewLatestTagReferenceName(name string) ReferenceName {
	return newReferenceName(name, latestTag)
}

// NewTagReferenceName creates a reference name for given ReferenceOpts.
func NewTagReferenceName(name string) ReferenceName {
	return newReferenceName(name, tag)
}

// isType tells if given reference is of asked type.
func (r ReferenceName) isType(typ referenceType) bool {
	prefix := string(typ) + separateDelimeter
	return strings.HasPrefix(string(r), prefix)
}

// IsLatestCommit tells if the reference is for latest commit.
func (r ReferenceName) IsLatestCommit() bool {
	return r.isType(latestCommit)
}

// IsLatestTag tells if the reference is for latest tag.
func (r ReferenceName) IsLatestTag() bool {
	return r.isType(latestTag)
}

// IsTag tells if the reference is for tag.
func (r ReferenceName) IsTag() bool {
	return r.isType(tag)
}

// Name returns the name of reference.
func (r ReferenceName) Name() string {
	opts := strings.Split(string(r), separateDelimeter)
	return opts[1]
}

// IsValid tells if it's a valid reference name.
func (r ReferenceName) IsValid() bool {
	opts := strings.Split(string(r), separateDelimeter)
	if len(opts) != 2 {
		return false
	}

	switch referenceType(opts[0]) {
	case latestCommit, latestTag, tag:
	default:
		return false
	}

	return true
}

// GitReferenceName returns the git's version of reference name.
func (r ReferenceName) GitReferenceName() plumbing.ReferenceName {
	if r.IsTag() {
		return plumbing.NewTagReferenceName(r.Name())
	}

	return plumbing.NewBranchReferenceName(r.Name())
}

// String returns the reference name as string.
func (r ReferenceName) String() string {
	return string(r)
}
