package caddygit

import (
	"testing"

	"github.com/caddyserver/caddy/v2"

	"github.com/vrongmeal/caddygit/utils"
)

func TestReplacer_Valid(t *testing.T) {
	repl := caddy.NewReplacer()
	addGitVarsToReplacer(repl)

	for i, tc := range []struct {
		input  string
		expect string
	}{
		{
			input:  "{git.ref.latest_commit}",
			expect: utils.NewLatestCommitReferenceName("").String(),
		},
		{
			input:  "{git.ref.latest_tag}",
			expect: utils.NewLatestTagReferenceName("").String(),
		},
		{
			input:  "{git.ref.branch.abc}",
			expect: utils.NewLatestCommitReferenceName("abc").String(),
		},
		{
			input:  "{git.ref.branch.abc.latest_commit}",
			expect: utils.NewLatestCommitReferenceName("abc").String(),
		},
		{
			input:  "{git.ref.branch.abc.latest_tag}",
			expect: utils.NewLatestTagReferenceName("abc").String(),
		},
		{
			input:  "{git.ref.tag.v1.2.3}",
			expect: utils.NewTagReferenceName("v1.2.3").String(),
		},
	} {
		actual, err := repl.ReplaceOrErr(tc.input, false, true)
		if err != nil {
			t.Errorf("Test %d: Failed with error for %s: %v", i, tc.input, err)
		}

		if actual != tc.expect {
			t.Errorf("Test %d: Expected placeholder %s to be '%s' but got '%s'",
				i, tc.input, tc.expect, actual)
		}
	}
}

func TestReplacer_Invalid(t *testing.T) {
	repl := caddy.NewReplacer()
	addGitVarsToReplacer(repl)

	for i, tc := range []string{
		"{git.ref.tag}",
		"{git.ref.tag.}",
		"{git.ref.branch}",
		"{git.ref.branch.latest_commit}",
		"{git.ref.branch.latest_tag}",
	} {
		_, err := repl.ReplaceOrErr(tc, false, true)
		if err == nil {
			t.Errorf("Test %d: Did not throw error for %s", i, tc)
		}
	}
}
