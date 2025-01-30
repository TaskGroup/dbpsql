package errors

import "errors"

var (
	ErrNotFound      = errors.New("data not found")
	ErrNotAuthorized = errors.New("data not authorized")
	ErrInternal      = errors.New("internal server error")
	ErrNotUpdated    = errors.New("not record for update")
	ErrAlreadyExists = errors.New("data already exists")
	ErrInvalidParams = errors.New("invalid parameters")
	ErrAccessDenied  = errors.New("access denied")
)

type SQLError struct {
	ErrMsg string `json:"err_msg" db:"err_msg"`
}
