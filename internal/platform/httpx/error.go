package httpx

import "github.com/gin-gonic/gin"

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

func NewErrorResponse(code string, message string) ErrorResponse {
	return ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
}

func WriteError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, NewErrorResponse(code, message))
}

func AbortError(c *gin.Context, status int, code string, message string) {
	c.AbortWithStatusJSON(status, NewErrorResponse(code, message))
}
