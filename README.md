# caddygit

> Git module for Caddy v2

Package caddygit implements a Git Caddy module. This module can be used to deploy
your website with a simple git push. This module starts a service that runs during
the lifetime of the server. When the service starts, it clones the repository. While
the server is still up, it pulls the latest every so often.

## Installation

Simply add the following import to [`cmd/caddy/main.go`](https://github.com/caddyserver/caddy/blob/master/cmd/caddy/main.go) and build the caddy binary:

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

```console
> xcaddy build v2.0.0-rc.1 \
    --with github.com/vrongmeal/caddygit
```

## API structure

```json
{
    // Your caddy apps.
	"apps": {
        // Git app module.
        "git": {
            // Your git clients to be deployed.
            "client": [
                // Example client.
                {
                    // Git repository info.
                    "repo": {
                        // URL (HTTP only) of the git repository.
                        "url": "http://github.com/vrongmeal/caddygit",

                        // Path to clone the repository in. If path specified exists and is a git
                        // repository, it simply opens the repo. If the path is not a repo and does
                        // not exist, it creates a repo in that path.
                        "path": "/path/to/clone",

                        // Remote name. Defaults to "origin".
                        "remote": "my-remote",

                        // Branch (or tag) of the repository to clone. Defaults to `master` if nothing is provided.
                        // Can be set using placeholders:
                        //  `{git.ref.branch.<branch>}` for branch name. Equivalent to `<branch>`.
                        //  `{git.ref.branch.<branch>.latest_commit}` is same as above.
                        //  `{git.ref.latest_commit}` is same as above for default branch. Equivalent to empty string.
                        //  `{git.ref.branch.<branch>.latest_tag}` fetches latest tag for given branch.
                        //  `{git.ref.latest_tag}` is same as above for default branch.
                        //  `{git.ref.tag.<tag>}` for tag name.
                        "branch": "my-branch",

                        // Username and Password for authentication of private repositories.
                        // If authenticating via access token, set the password equal to the value of
                        // access token and username can be omitted.
                        "username": "vrongmeal",
                        "password": "password",

                        // SingleBranch specifies whether to clone only the specified branch.
                        "single_branch": true,

                        // Depth of commits to fetch.
                        "depth": 1
                    },
                    // Service info.
                    "service": {
                        // Type of the service. Can be "time" or "webhook". Defaults to "time".
                        // WIP: webhook service.
                        "type": "time",

                        // TimeService options:-

                        // Interval after which service will tick. Defaults to 1 hour.
                        "interval": "10m"
                    },
                    // Commands to run after every update.
                    "commands_after": [
                        {
                            // Command to execute.
                            "command": ["echo", "hello world"],

                            // Whether to run command in background (async). Defaults to false.
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
