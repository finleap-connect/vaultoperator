package vault

import (
	"errors"
)

var (
	ErrTimeout               = errors.New("did not receive vault token within time")
	ErrAuthMethodNotProvided = errors.New("method not provided")
	ErrMissingToken          = errors.New("missing client token")
	ErrNotFound              = errors.New("not found")
)
