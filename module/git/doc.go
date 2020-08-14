// Package git implements the Caddy Git Module.
//
// The module is helpful in creating git clients that pull from the given
// repository at regular intervals of time (poll service) or whenever there
// is a change in the repository (webhook service). On a successful pull
// it runs the specified commands to automate deployment.
package git
