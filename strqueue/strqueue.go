package strqueue

// Q implements a queue of strings
type Q struct {
	elems       []string
	start, size int
}

// NewQ creates and returns an Q
func NewQ() *Q {
	return &Q{
		elems: make([]string, 10), // seems like an OK initial size
	}
}

// Push adds an element to the tail of 'q'
func (q *Q) Push(s string) {
	if q.size == len(q.elems) {
		// copy elems to new, double-size slice
		newElems := make([]string, len(q.elems)*2)
		n := copy(newElems, q.elems[q.start:])
		copy(newElems[n:], q.elems[:q.start])

		// reset start and elem slice
		q.start = 0
		q.elems = newElems
	}
	p := (q.start + q.size) % len(q.elems)
	q.elems[p] = s
	q.size++
}

// Pop removes an element from the head of 'q' and returns it. If q.Size() > 0,
// then 'ok' is 'true', otherwise 'val' is 0 and 'ok' is 'false'
func (q *Q) Pop() (val string, ok bool) {
	if q.size == 0 {
		return "", false
	}
	result := q.elems[q.start]
	q.start = (q.start + 1) % len(q.elems)
	q.size--
	if q.size == 0 {
		q.start = 0 // might as well jump back to the beginning
	}
	return result, true
}

// Size returns the number of elements in 'q'
func (q *Q) Size(i int) int {
	return q.size
}
