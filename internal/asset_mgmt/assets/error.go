package assets

import (
	"errors"
	"fmt"
	"net/http"
)

// ===== Error model =====
// ドメイン全体で共通のコードを定義
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

// Error インターフェースの実装
func (e *APIError) Error() string      { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// エラー生成用ヘルパー関数
func ErrInvalid(msg string) *APIError  { return &APIError{Code: CodeInvalidArgument, Message: msg} }
func ErrNotFound(msg string) *APIError { return &APIError{Code: CodeNotFound, Message: msg} }
func ErrConflict(msg string) *APIError { return &APIError{Code: CodeConflict, Message: msg} }
func ErrInternal(msg string) *APIError { return &APIError{Code: CodeInternal, Message: msg} }

// HTTPステータスコードへの変換ロジック
func toHTTPStatus(err error) int {
	var api *APIError
	if errors.As(err, &api) {
		switch api.Code {
		case CodeInvalidArgument:
			return http.StatusBadRequest
		case CodeNotFound:
			return http.StatusNotFound
		case CodeConflict:
			return http.StatusConflict
		case CodeInternal:
			return http.StatusInternalServerError
		}
	}
	return http.StatusInternalServerError
}