package controllers

import (
	"errors"
)

var (
	ErrUnknownGenerator     = errors.New("no generator specified")
	ErrInvalidGeneratorArgs = errors.New("invalid arguments for specified generator")
	ErrInvalidVaultPath     = errors.New("invalid vault path, shoud contain at least 3 segments")
	ErrPermissionDenied     = errors.New("permission denied")
)
