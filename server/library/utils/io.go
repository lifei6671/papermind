package utils

import (
	"io"
	"log/slog"
)

// SafeClose 安全关闭 io.Closer 实现，忽略错误。
func SafeClose(closer io.Closer) {
	if closer != nil {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("safe close failed", "errmsg", err)
			}
		}()
		_ = closer.Close()
	}
}
