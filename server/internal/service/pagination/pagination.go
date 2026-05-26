package pagination

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type Input struct {
	Page     int
	PageSize int
}

type Result[T any] struct {
	Items    []T
	Page     int
	PageSize int
	Total    int64
}

func Normalize(input Input) Input {
	if input.Page <= 0 {
		input.Page = DefaultPage
	}
	if input.PageSize <= 0 {
		input.PageSize = DefaultPageSize
	}
	if input.PageSize > MaxPageSize {
		input.PageSize = MaxPageSize
	}
	return input
}

func Offset(input Input) int {
	input = Normalize(input)
	return (input.Page - 1) * input.PageSize
}
