package strqueue

import (
	"fmt"
	"log"
	"strconv"
	"testing"
)

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

func fatalf(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
}

func TestBasic(t *testing.T) {
	q := NewQ()
	for N := 0; N < 50; N++ {
		for i := 0; i < 9; i++ {
			q.Push(strconv.Itoa(i + (N * 9)))
		}
		for i := 0; i < 8; i++ {
			expected := strconv.Itoa(i + (N * 8))
			if n, ok := q.Pop(); !ok || n != expected {
				fatalf("expected %d but got %d (ok: %t)\n", expected, n, ok)
			}
		}
	}
}
