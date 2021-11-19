package git

import (
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	httpcaddyfile.RegisterHandlerDirective("git", parseCaddyfileHandler)
}

// parseWebdav parses the Caddyfile tokens for the git directive.
func parseCaddyfileHandler(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	wd := new(Handler)
	err := wd.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}
	return wd, nil
}

// UnmarshalCaddyfile sets up the handler from Caddyfile tokens.
//
//    git [<repo path>] {
//        repo    <repo>
//        path    <path>
//        branch  <branch>
//    }
//
func (wd *Handler) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {

		if d.NextArg() {
			// Repo URL
			wd.Repository.URL = d.Val()
			if d.NextArg() {
				// Repo location
				wd.Repository.Path = d.Val()
				// No more args allowed
				if d.NextArg() {
					return d.ArgErr()
				}
			}
		}

		for d.NextBlock(0) {
			switch d.Val() {
			case "repo":
				fallthrough
			case "url":
				if wd.Repository.URL != "" {
					return d.Err("repo url already specified")
				}
				if !d.NextArg() {
					return d.ArgErr()
				}
				wd.Repository.URL = d.Val()
			case "path":
				if wd.Repository.Path != "" {
					return d.Err("root path already specified")
				}
				if !d.NextArg() {
					return d.ArgErr()
				}
				wd.Repository.Path = d.Val()
			case "branch":
				if wd.Repository.Branch != "" {
					return d.Err("prefix already specified")
				}
				if !d.NextArg() {
					return d.ArgErr()
				}

				wd.Repository.Branch = d.Val()
			default:
				return d.Errf("unrecognized subdirective: %s", d.Val())
			}
		}
	}
	return nil
}
