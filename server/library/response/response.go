package response

import "github.com/lifei6671/papermind/server/library/code"

type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type PageBody[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

func OK(data any) Body {
	return Body{
		Code:    code.Success,
		Message: code.Message(code.Success),
		Data:    data,
	}
}

func Fail(errorCode int, message string) Body {
	if message == "" {
		message = code.Message(errorCode)
	}
	return Body{
		Code:    errorCode,
		Message: message,
		Data:    nil,
	}
}

func Page[T any](items []T, page int, pageSize int, total int64) PageBody[T] {
	if items == nil {
		items = []T{}
	}
	return PageBody[T]{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}
}
