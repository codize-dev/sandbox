package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
)

// ErrorCode is a typed string for structured error codes.
type ErrorCode string

const (
	CodeInvalidRequestBody  ErrorCode = "INVALID_REQUEST_BODY"
	CodeValidationError     ErrorCode = "VALIDATION_ERROR"
	CodeInternalError       ErrorCode = "INTERNAL_ERROR"
	CodeTimeout             ErrorCode = "TIMEOUT"
	CodeNotFound            ErrorCode = "NOT_FOUND"
	CodeMethodNotAllowed    ErrorCode = "METHOD_NOT_ALLOWED"
	CodeRequestBodyTooLarge ErrorCode = "REQUEST_BODY_TOO_LARGE"
	CodeServerBusy          ErrorCode = "SERVER_BUSY"
)

// Message returns the canonical human-readable message for the error code.
func (c ErrorCode) Message() string {
	switch c {
	case CodeInvalidRequestBody:
		return "invalid request body"
	case CodeValidationError:
		return "request validation failed"
	case CodeInternalError:
		return "internal server error"
	case CodeTimeout:
		return "execution timed out"
	case CodeNotFound:
		return "the requested resource was not found"
	case CodeMethodNotAllowed:
		return "the request method is not allowed for this resource"
	case CodeRequestBodyTooLarge:
		return "request body too large"
	case CodeServerBusy:
		return "server is busy, please retry later"
	default:
		return "internal server error"
	}
}

// ErrorResponse is the unified JSON error response body.
type ErrorResponse struct {
	Code    ErrorCode         `json:"code"`
	Message string            `json:"message"`
	Errors  []ValidationError `json:"errors,omitempty"`
}

// ValidationError describes a field-level validation failure.
type ValidationError struct {
	Path    []any  `json:"path"`
	Message string `json:"message"`
}

// NewHTTPErrorHandler returns a custom echo.HTTPErrorHandler that renders
// framework-level errors (404, 405, etc.) in the unified ErrorResponse format.
func NewHTTPErrorHandler() echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		if r, uErr := echo.UnwrapResponse(c.Response()); uErr == nil && r.Committed {
			return
		}

		code := http.StatusInternalServerError
		var sc echo.HTTPStatusCoder
		if errors.As(err, &sc) {
			if tmp := sc.StatusCode(); tmp != 0 {
				code = tmp
			}
		}

		var resp ErrorResponse
		switch code {
		case http.StatusNotFound:
			resp = ErrorResponse{
				Code:    CodeNotFound,
				Message: CodeNotFound.Message(),
			}
		case http.StatusMethodNotAllowed:
			resp = ErrorResponse{
				Code:    CodeMethodNotAllowed,
				Message: CodeMethodNotAllowed.Message(),
			}
		case http.StatusRequestEntityTooLarge:
			resp = ErrorResponse{
				Code:    CodeRequestBodyTooLarge,
				Message: CodeRequestBodyTooLarge.Message(),
			}
		default:
			resp = ErrorResponse{
				Code:    CodeInternalError,
				Message: CodeInternalError.Message(),
			}
		}

		if c.Request().Method == http.MethodHead {
			_ = c.NoContent(code)
		} else {
			_ = c.JSON(code, resp)
		}
	}
}
