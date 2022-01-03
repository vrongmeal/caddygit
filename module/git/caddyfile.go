package git

import (
	"encoding/json"
	"strconv"

	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/vrongmeal/caddygit"
	"github.com/vrongmeal/caddygit/module"
)

type CaddyfileSettings struct {
	Repository  caddygit.RepositoryOpts
	RawCommands []caddygit.Command
	ServiceRaw  json.RawMessage

	// Webhook settings
	HookSecret string
	HookRaw    json.RawMessage
}

type ServiceRaw struct {
	Type     string `json:"type,omitempty"`
	Interval string `json:"interval,omitempty"`
}

func init() {
	httpcaddyfile.RegisterGlobalOption("git", parseGlobalCaddyfileBlock)
	httpcaddyfile.RegisterHandlerDirective("git", parseCaddyfileHandlerBlock)
}

// parseGlobalCaddyfileBlock parses the Caddyfile tokens for the global git directive.
func parseGlobalCaddyfileBlock(d *caddyfile.Dispenser, prev interface{}) (interface{}, error) {
	var git App

	// decode the existing value and merge to it.
	if prev != nil {
		if app, ok := prev.(httpcaddyfile.App); ok {
			if err := json.Unmarshal(app.Value, &git); err != nil {
				return nil, d.Errf("internal error: %v", err)
			}
		}
	}

	// Parse directive
	client, err := newClientFromDispenser(d)
	if err != nil {
		return nil, err
	}

	// append repo to global git app.
	git.Clients = append(git.Clients, client)

	// tell Caddyfile adapter that this is the JSON for an app
	return httpcaddyfile.App{
		Name:  "git",
		Value: caddyconfig.JSON(git, nil),
	}, nil
}

// parseCaddyfileHandlerBlock parses the Caddyfile tokens for the git directive.
func parseCaddyfileHandlerBlock(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	handler, err := newHandlerFromDispenser(h.Dispenser)
	return &handler, err
}

func newClientFromDispenser(d *caddyfile.Dispenser) (client module.Client, err error) {
	var config CaddyfileSettings
	err = config.UnmarshalCaddyfile(d)
	client.RepositoryOpts = config.Repository
	client.RawCommands = config.RawCommands
	client.ServiceRaw = config.ServiceRaw
	return
}

func newHandlerFromDispenser(d *caddyfile.Dispenser) (handler Handler, err error) {
	var config CaddyfileSettings
	err = config.UnmarshalCaddyfile(d)
	handler.Repository = config.Repository
	handler.Commands = config.RawCommands
	handler.Secret = config.HookSecret
	handler.HookRaw = config.HookRaw
	return
}

// UnmarshalCaddyfile sets up the handler from Caddyfile tokens.
//
//    git repo [path]
//
// For more control use the following syntax:
//    git [<matcher>] [<repo>] [<path>] {
//        repo|url          <repo>
//        path              <path>
//        branch            <branch>
//        auth_user         <username>
//        auth_secret       <password>
//        single_branch     true|false
//        depth             <depth>
//        service_type      <service type>
//        service_interval  <service interval>
//        webhook_secret    <secret>
//        webhook_service   <service info>
//        command_after     <command>
//        command_async     true|false
//    }
//
func (config *CaddyfileSettings) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		var serviceConfig ServiceRaw
		var command caddygit.Command

		if d.NextArg() {
			// Repo URL
			config.Repository.URL = d.Val()
			if d.NextArg() {
				// Repo location
				config.Repository.Path = d.Val()
				// No more args allowed
				if d.NextArg() {
					return d.ArgErr()
				}
			}
		}

		for d.NextBlock(0) {
			var err error

			switch d.Val() {
			case "repo":
				// Retro-compatibility with Caddy Git v1
				fallthrough
			case "url":
				err = validateStringParamter(d, "repo or url", &config.Repository.URL)
			case "path":
				err = validateStringParamter(d, "path", &config.Repository.Path)
			case "branch":
				err = validateStringParamter(d, "branch", &config.Repository.Branch)
			case "auth_user":
				err = validateStringParamter(d, "auth_user", &config.Repository.Username)
			case "auth_secret":
				err = validateStringParamter(d, "auth_secret", &config.Repository.Password)
			case "single_branch":
				if !d.NextArg() {
					return d.ArgErr()
				}
				b, err := strconv.ParseBool(d.Val())
				if err != nil {
					return err
				}
				config.Repository.SingleBranch = b
			case "depth":
				if !d.NextArg() {
					return d.ArgErr()
				}
				i, err := strconv.ParseInt(d.Val(), 10, 0)
				if err != nil {
					return err
				}
				config.Repository.Depth = int(i)
			case "service_type":
				err = validateStringParamter(d, "service_type", &serviceConfig.Type)
			case "service_interval":
				err = validateStringParamter(d, "service_interval", &serviceConfig.Interval)
			case "webhook_secret":
				err = validateStringParamter(d, "webhook_secret", &config.HookSecret)
			case "webhook_service":
				var webhook_service string
				err = validateStringParamter(d, "webhook_service", &webhook_service)
				config.HookRaw = json.RawMessage(webhook_service)
			case "command_after":
				if len(command.Args) > 0 {
					return d.Err("command_after already specified")
				}
				args := d.RemainingArgs()
				if len(args) < 1 {
					return d.Err("command_after not specified")
				}
				command.Args = args
			case "command_async":
				if !d.NextArg() {
					return d.ArgErr()
				}
				b, err := strconv.ParseBool(d.Val())
				if err != nil {
					return err
				}
				command.Async = b
			default:
				return d.Errf("unrecognized subdirective: %s", d.Val())
			}

			if err != nil {
				return err
			}

		}

		// Set ServiceRaw config
		config.ServiceRaw = caddyconfig.JSON(serviceConfig, nil)
		if string(config.ServiceRaw) == `{}` {
			config.ServiceRaw = nil
		}

		// Set command
		config.RawCommands = []caddygit.Command{command}
	}

	return nil
}

func validateStringParamter(d *caddyfile.Dispenser, name string, param *string) error {
	if *param != "" {
		return d.Err(name + " already specified")
	}
	if !d.NextArg() {
		return d.ArgErr()
	}
	*param = d.Val()
	return nil
}
