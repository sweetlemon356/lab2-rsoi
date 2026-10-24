package application

import (
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("not found")

type FieldError struct{ Field, Error string }

type ValidationError struct{ Fields []FieldError }

func (e *ValidationError) Error() string { return "invalid request" }

type DependencyError struct {
	Service string
	Err     error
}

func (e *DependencyError) Error() string { return fmt.Sprintf("%s: %v", e.Service, e.Err) }
func (e *DependencyError) Unwrap() error { return e.Err }
