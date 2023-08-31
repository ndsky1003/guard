package guard

import (
	"errors"
	"testing"

	"github.com/ndsky1003/guard/options"
)

func TestGuard(t *testing.T) {
	identider := "dddd"
	_ = Check(identider)
	if err := Check(identider, options.Guard().SetErr(errors.New("err1"))); err != nil {
		t.Error(err)
	}
	// Release(identider)
	if err := Check(identider, options.Guard().SetErr(errors.New("err2"))); err != nil {
		t.Error(err)
	}
	Release(identider)
}
