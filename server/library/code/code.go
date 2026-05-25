package code

const (
	Success       = 0
	InvalidParam  = 40001
	InternalError = 50000
)

var messages = map[int]string{
	Success:       "ok",
	InvalidParam:  "参数错误",
	InternalError: "系统内部错误",
}

func Message(value int) string {
	if message, ok := messages[value]; ok {
		return message
	}
	return "未知错误"
}
