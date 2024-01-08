# caddygit

_**NOTE: This repository is no longer maintained. Kindly use [greenpau/caddy-git](https://github.com/greenpau/caddy-git)**_

> Git module for Caddy v2

The module is helpful in creating git clients that pull from the given
repository at regular intervals of time (poll service) or whenever there
is a change in the repository (webhook service). On a successful pull
it runs the specified commands to automate deployment.

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
	_ "github.com/vrongmeal/caddygit/module/git" // Yay!!!
)

func main() {
	caddycmd.Main()
}
```

**OR** you can use [xcaddy](https://github.com/caddyserver/xcaddy) to build:

```bash
$ xcaddy build v2.4.6 \
    --with github.com/vrongmeal/caddygit/module/git
```

## API structure

As a top level app for global service.

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
                        // Type of the service.
                        // Services supported: poll, webhook
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
As an handler within a route.

```jsonc
{
    ...
    "routes": [
        {
            "handle": [
                // exec configuration for an endpoint route
                {
                    // required to inform caddy the handler is `exec`
                    "handler": "git",

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
                    
                    // Webhook secret
                    "secret": "secret",

                    // Webhook service info
                    "hook": "",

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
            ],
            "match": [
                {
                    "path": [
                        "/update"
                    ]
                }
            ]
        }
    ]
}
```

## Caddyfile

For a seamless transition from [Git module for Caddy v1](https://github.com/abiosoft/caddy-git), support for Caddyfile was added in a similar fashion:

    git repo [path]

For more control use the following syntax (bear in mind, this options are different from v1):

    git [<matcher>] [<repo>] [<path>] {
        repo|url          <repo>
        path              <path>
        branch            <branch>
        auth_user         <username>
        auth_secret       <password>
        single_branch     true|false
        depth             <depth>
        service_type      <service type>
        service_interval  <service interval>
        webhook_secret    <secret>
        webhook_service   <service info>
        command_after     <command>
        command_async     true|false
    }

- **matcher** - [Caddyfile matcher](https://caddyserver.com/docs/caddyfile/matchers). When set, this command runs when there is an http request at the current route or the specified matcher. You may leverage other matchers to protect the endpoint. Webhook URL to update the git repository.
- **repo** is the URL to the repository
- **path** is the path to clone the repository into; default is site root. It can be absolute or relative (to site root).
- **branch** is the branch or tag to pull; default is master branch.

Here is an example:

    {
        order git before file_server
    }
    localhost:8000 {
        git /update "http://github.com/vrongmeal/caddygit" /caddygit
        file_server {
            browse
            root /caddygit
        }
    }

## TODO:

- [X] Support for Caddyfile
- [X] Webhook service
