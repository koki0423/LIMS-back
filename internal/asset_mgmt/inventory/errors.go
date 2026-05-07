package inventory

import "errors"

var (
	ErrInvalidQuantity   = errors.New("quantity must be > 0")
	ErrInsufficientStock = errors.New("insufficient stock")
)
