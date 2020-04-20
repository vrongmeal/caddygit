# caddygit

> Git module for Caddy v2

Package caddygit implements a Git Caddy module. This module can be used to deploy
your website with a simple git push. This module starts a service that runs during
the lifetime of the server. When the service starts, it clones the repository. While
the server is still up, it pulls the latest every so often.

## Installation

Simply add the following import to
[`cmd/caddy/main.go`](https://github.com/caddyserver/caddy/blob/master/cmd/caddy/main.go)
and build the caddy binary:

```go
package main

import (
	caddycmd "github.com/caddyserver/caddy/v2/cmd"

	// plug in Caddy modules here
    _ "github.com/caddyserver/caddy/v2/modules/standard"
    _ "github.com/vrongmeal/caddygit" // Yay!!!
)

func main() {
	caddycmd.Main()
}
```

**OR** you can use [xcaddy](https://github.com/caddyserver/xcaddy) to build:

```bash
$ xcaddy build v2.0.0-rc.1 \
    --with github.com/vrongmeal/caddygit
```

## API structure

```jsonc
{
    // Your caddy apps.
	"apps": {
        // Git app module.
        "git": {
            // Your git clients to be deployed.
            "clients": [
                // Example client.
                {
                    // Git repository info.
                    "repo": {
                        // HTTP URL of the git repository.
                        "url": "http://github.com/vrongmeal/caddygit",

                        // Path to clone the repository in. If path specified
                        // exists and is a git repository, it simply opens the
                        // repo. If the path is not a repo and does not exist,
                        // it creates a repo in that path.
                        "path": "/path/to/clone",

                        // Branch (or tag) of the repository to clone. Defaults
                        // to `master`.
                        "branch": "my-branch",

                        // Username and secret for authentication of private
                        // repositories. If authenticating via access token,
                        // set the auth_secret equal to the value of access token
                        // and auth_user can be omitted.
                        "auth_user": "vrongmeal",
                        "auth_secret": "password",

                        // Specifies whether to clone only the specified branch.
                        "single_branch": true,

                        // Depth of commits to fetch.
                        "depth": 1
                    },
                    // Service info.
                    "service": {
                        // Type of the service. Currently only "poll" supported.
                        "type": "poll",

                        // Interval after which service will tick.
                        "interval": "10m"
                    },
                    // Commands to run after every update.
                    "commands_after": [
                        {
                            // Command to execute.
                            "command": ["echo", "hello world"],

                            // Whether to run command in background (async).
                            // Defaults to false.
                            "async": true
                        }
                    ]
                }
            ]
        }
    }
}
```

## TODO:

- [ ] Support for Caddyfile
- [ ] Webhook service
