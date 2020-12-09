package runalyze

import "strings"

// Errors contains a list of errors
type Errors []string

func (e Errors) Error() string {
	return strings.Join(e, ",")
}

// Error specifies additional methods on the standard error interface
type Error interface {
	error
	Temporary() bool
}

// AuthError indicates an issue with the authentication token
type AuthError string

func (e AuthError) Error() string { return string(e) }

// Temporary indicates if an error is temporary
func (e AuthError) Temporary() bool { return false }
