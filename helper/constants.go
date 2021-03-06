package helper

import (
	"errors"
	"time"
)

const (
	// DefaultRecoveryInterval specifies the default recovery interval for
	// client instances.
	DefaultRecoveryInterval = time.Second * 30
)

var (
	// ErrDisconnected is returned when a server is offline.
	ErrDisconnected = errors.New("The server is disconnected or offline.")
	// ErrClosed is returned from calls to a service or interface in the event
	// that the Close() function has already been called.
	ErrClosed = errors.New("Interface is closing or already closed.")
	// ErrZeroWorkers is returned when zero workers are specified in a call to
	// NewLimiter.
	ErrZeroWorkers = errors.New("Zero workers were specified for the limiter.")
)
