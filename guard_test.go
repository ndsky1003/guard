package guard

import (
	"errors"
	"testing"
	"time"
)

func TestDcc(t *testing.T) {
	t.Log("dd")
	t.Error("ddddd")
	gt := NewGuardTime(5*time.Second, errors.New("操作过多"))
	t.Error(gt)
	for i := 0; i < 100; i++ {
		t.Log("dd")
		t.Error("ddd")
		if err := gt.Handle("cc"); err != nil {
			t.Error(err)
		}
	}
	t.Error("done")

}
