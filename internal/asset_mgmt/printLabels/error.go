package printLabels

import (
	"errors"
	"fmt"
)

// ===== Error model =====
type Code string

const (
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT"
	CodeInternal        Code = "INTERNAL"
)

type APIError struct {
	Code    Code
	Message string
}

func (e *APIError) Error() string      { return fmt.Sprintf("%s: %s", e.Code, e.Message) }
func ErrInvalid(msg string) *APIError  { return &APIError{Code: CodeInvalidArgument, Message: msg} }
func ErrNotFound(msg string) *APIError { return &APIError{Code: CodeNotFound, Message: msg} }
func ErrConflict(msg string) *APIError { return &APIError{Code: CodeConflict, Message: msg} }
func ErrInternal(msg string) *APIError { return &APIError{Code: CodeInternal, Message: msg} }

func toHTTPStatus(err error) int {
	var api *APIError
	if errors.As(err, &api) {
		switch api.Code {
		case CodeInvalidArgument:
			return 400
		case CodeNotFound:
			return 404
		case CodeConflict:
			return 409
		default:
			return 500
		}
	}
	return 500
}