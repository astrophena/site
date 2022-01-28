// Package env contains definitions for the environments in which site can run.
package env

// Env is the environment in which site can run.
type Env string

// Available environments.
const (
	Dev     = Env("dev")
	Staging = Env("staging")
	Prod    = Env("prod")
)
