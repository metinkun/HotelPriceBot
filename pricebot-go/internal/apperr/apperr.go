// Package apperr HTTP durum koduna eslenebilen uygulama hatalari.
package apperr

import (
	"errors"
	"fmt"
)

type Kind int

const (
	Internal Kind = iota
	BadRequest
	NotFound
	Conflict
	Unauthorized
	Forbidden
	TooMany
	Unavailable
)

type Error struct {
	Kind    Kind
	Message string
	Details any
}

func (e *Error) Error() string { return e.Message }

func New(kind Kind, format string, args ...any) *Error {
	return &Error{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

func NotFoundf(format string, args ...any) *Error   { return New(NotFound, format, args...) }
func BadRequestf(format string, args ...any) *Error { return New(BadRequest, format, args...) }
func Conflictf(format string, args ...any) *Error   { return New(Conflict, format, args...) }

// WithDetails hataya ek ayrinti (orn. provider bazli cozumleme hatalari) ekler.
func (e *Error) WithDetails(d any) *Error {
	e.Details = d
	return e
}

// Status hatanin HTTP durum kodunu doner; bilinmeyen hatalar 500.
func Status(err error) int {
	var e *Error
	if errors.As(err, &e) {
		switch e.Kind {
		case BadRequest:
			return 400
		case Unauthorized:
			return 401
		case NotFound:
			return 404
		case Conflict:
			return 409
		case Forbidden:
			return 403
		case TooMany:
			return 429
		case Unavailable:
			return 503
		}
	}
	return 500
}

// DetailsOf varsa hatanin Details alanini doner.
func DetailsOf(err error) any {
	var e *Error
	if errors.As(err, &e) {
		return e.Details
	}
	return nil
}
