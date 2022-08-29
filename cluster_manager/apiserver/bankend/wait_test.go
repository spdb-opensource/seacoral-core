package bankend

import (
	"errors"
	"testing"
	"time"
)

func update(err error) error {
	return nil
}

func TestWaitTask(t *testing.T) {
	n := 0
	wt := NewWaitTask(time.Second, update)

	go wt.WithTimeout(time.Second*5, func() (bool, error) {
		n++

		if n%2 == 0 {
			return false, errors.New("error-test")
		}

		return false, nil
	})

	time.Sleep(6 * time.Second)

	if n < 5 {
		t.Error("unexpect error", n)
	}
}
