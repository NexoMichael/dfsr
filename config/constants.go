package config

import "errors"

const updateChanSize = 16

var (
	// ErrClosed is returned from calls to a service or interface in the event
	// that the Close() function has already been called.
	ErrClosed = errors.New("Interface is closing or already closed.")

	// ErrDomainLookupFailed is returned when the appropriate domain naming
	// context cannot be determined.
	ErrDomainLookupFailed = errors.New("Unable to determine DFSR configuration domain.")
)
