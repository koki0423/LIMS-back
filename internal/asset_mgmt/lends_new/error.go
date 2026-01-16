package lends_new

import "fmt"

type DomainError struct {
	Code    string
	Message string
}

func (e *DomainError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// 共通エラーコード（必要に応じて追加）
const (
	ErrCodeNotFound           = "NOT_FOUND"
	ErrCodeInvalidArgument    = "INVALID_ARGUMENT"
	ErrCodeConflict           = "CONFLICT"
	ErrCodeInternal           = "INTERNAL"
	ErrCodeQuantityOverReturn = "QUANTITY_OVER_RETURN"
)

func NewNotFoundError(msg string) error {
	return &DomainError{
		Code:    ErrCodeNotFound,
		Message: msg,
	}
}

func NewInvalidArgumentError(msg string) error {
	return &DomainError{
		Code:    ErrCodeInvalidArgument,
		Message: msg,
	}
}

func NewConflictError(msg string) error {
	return &DomainError{
		Code:    ErrCodeConflict,
		Message: msg,
	}
}

func NewQuantityOverReturnError() error {
	return &DomainError{
		Code:    ErrCodeQuantityOverReturn,
		Message: "return quantity exceeds lent quantity",
	}
}
