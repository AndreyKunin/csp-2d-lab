package csp2d

import (
	"csp2d/util"
	"sync/atomic"
)

// PostfixArena is a bump allocator for []int slices that share a lifetime.
// All slices allocated from one arena are released together via Reset().
//
// IMPORTANT: PostfixArena is NOT goroutine-safe. It is intended to be owned
// by a single Csp2dProblemImpl (which itself is owned by a single
// SimulationJob running in its own goroutine).
type PostfixArena struct {
	buf      []int
	cursor   int
	overflow uint64
	name     string
}

func NewPostfixArena(name string, capacityInts int) *PostfixArena {
	if capacityInts < 0 {
		capacityInts = 0
	}
	return &PostfixArena{
		buf:  make([]int, capacityInts),
		name: name,
	}
}

// Alloc returns a slice with len=0, cap=n carved out of the arena.
// On overflow, falls back to make() and logs a warning (throttled).
func (a *PostfixArena) Alloc(n int) []int {
	if n <= 0 {
		return nil
	}
	if a.cursor+n > len(a.buf) {
		count := atomic.AddUint64(&a.overflow, 1)
		if count <= 8 || (count&(count-1)) == 0 {
			util.Warning.Printf(
				"PostfixArena[%s] overflow: requested=%d cursor=%d capacity=%d totalOverflows=%d — falling back to make()",
				a.name, n, a.cursor, len(a.buf), count,
			)
		}
		return make([]int, 0, n)
	}
	s := a.buf[a.cursor : a.cursor : a.cursor+n]
	a.cursor += n
	return s
}

func (a *PostfixArena) AllocFilled(src []int) []int {
	if len(src) == 0 {
		return nil
	}
	s := a.Alloc(len(src))
	s = append(s, src...)
	return s
}

func (a *PostfixArena) Reset() {
	// Zero out for safety: any stale slices that someone forgot will read zeros
	// instead of stale data, surfacing bugs faster.
	for i := 0; i < a.cursor; i++ {
		a.buf[i] = 0
	}
	a.cursor = 0
}

// Grow doubles the capacity if too small. Should be called only when the
// arena is empty (cursor == 0). After Grow, all previously returned slices
// from this arena are invalid.
func (a *PostfixArena) Grow(minCapacity int) {
	if minCapacity <= len(a.buf) {
		return
	}
	newCap := len(a.buf) * 2
	if newCap < minCapacity {
		newCap = minCapacity
	}
	util.Info.Printf(
		"PostfixArena[%s] growing capacity %d -> %d",
		a.name, len(a.buf), newCap,
	)
	a.buf = make([]int, newCap)
	a.cursor = 0
}

func (a *PostfixArena) Stats() (used, capacity int, overflow uint64) {
	return a.cursor, len(a.buf), atomic.LoadUint64(&a.overflow)
}

func (a *PostfixArena) Capacity() int {
	return len(a.buf)
}

func (a *PostfixArena) OverflowCount() uint64 {
	return atomic.LoadUint64(&a.overflow)
}

func (a *PostfixArena) ResetOverflowCount() {
	atomic.StoreUint64(&a.overflow, 0)
}
