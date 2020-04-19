package caddygit

import (
	"strings"

	"github.com/caddyserver/caddy/v2"

	"github.com/vrongmeal/caddygit/utils"
)

// Replacer prefixes and suffixes.
const (
	replacerMainPrefix      = "git."
	replacerRefPrefix       = replacerMainPrefix + "ref."
	replacerRefBranchPrefix = replacerRefPrefix + "branch."
	replacerRefTagPrefix    = replacerRefPrefix + "tag."

	replacerRefLatestTagSuffix    = ".latest_tag"
	replacerRefLatestCommitSuffix = ".latest_commit" // default behavior
)

func addGitVarsToReplacer(repl *caddy.Replacer) {
	repl.Map(gitReplacerFunc)
}

func gitReplacerFunc(key string) (interface{}, bool) {
	if strings.HasPrefix(key, replacerRefPrefix) {
		return gitRefReplacerFunc(key)
	}

	return "", false
}

func gitRefReplacerFunc(key string) (interface{}, bool) {
	if strings.HasPrefix(key, replacerRefTagPrefix) {
		tagName := key[len(replacerRefTagPrefix):]
		if tagName == "" {
			return "", false
		}

		return utils.NewTagReferenceName(tagName).String(), true
	}

	if strings.HasPrefix(key, replacerRefBranchPrefix) {
		branchName := key

		if strings.HasSuffix(key, replacerRefLatestTagSuffix) {
			branchName = key[:len(key)-len(replacerRefLatestTagSuffix)]
			if len(branchName) <= len(replacerRefBranchPrefix) {
				return "", false
			}

			branchName = branchName[len(replacerRefBranchPrefix):]
			return utils.NewLatestTagReferenceName(branchName).String(), true
		}

		if strings.HasSuffix(key, replacerRefLatestCommitSuffix) {
			branchName = key[:len(key)-len(replacerRefLatestCommitSuffix)]
			if len(branchName) <= len(replacerRefBranchPrefix) {
				return "", false
			}
		}

		branchName = branchName[len(replacerRefBranchPrefix):]
		return utils.NewLatestCommitReferenceName(branchName).String(), true
	}

	if strings.HasSuffix(key, replacerRefLatestTagSuffix) {
		branchName := key[:len(key)-len(replacerRefLatestTagSuffix)][len(replacerRefPrefix)-1:]
		if branchName != "" {
			return "", false
		}

		return utils.NewLatestTagReferenceName("").String(), true
	}

	if strings.HasSuffix(key, replacerRefLatestCommitSuffix) {
		branchName := key[:len(key)-len(replacerRefLatestCommitSuffix)][len(replacerRefPrefix)-1:]
		if branchName != "" {
			return "", false
		}

		return utils.NewLatestCommitReferenceName("").String(), true
	}

	return "", false
}
