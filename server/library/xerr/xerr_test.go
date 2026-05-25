package xerr

import (
	"errors"
	"testing"

	"github.com/lifei6671/papermind/server/library/code"
)

func TestNewCreatesBusinessError(t *testing.T) {
	err := New(code.InvalidParam, "参数错误")

	var bizErr *Error
	if !errors.As(err, &bizErr) {
		t.Fatalf("error type = %T", err)
	}
	if bizErr.Code != code.InvalidParam {
		t.Fatalf("Code = %d", bizErr.Code)
	}
	if bizErr.Message != "参数错误" {
		t.Fatalf("Message = %q", bizErr.Message)
	}
	if err.Error() != "参数错误" {
		t.Fatalf("Error() = %q", err.Error())
	}
}
