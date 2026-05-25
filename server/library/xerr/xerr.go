package xerr

import "github.com/lifei6671/papermind/server/library/code"

type Error struct {
	Code    int
	Message string
}

func New(errorCode int, message string) error {
	if message == "" {
		message = code.Message(errorCode)
	}
	return &Error{
		Code:    errorCode,
		Message: message,
	}
}

func (e *Error) Error() string {
	return e.Message
}
