package csp2d

import (
	"csp2d/util"
	"sync/atomic"
	"unsafe"
)

// PostfixArena is a bump allocator for []int slices that share a lifetime.
// All slices allocated from one arena are released together via Reset().
//
// Its buffer is reserved from the process-wide arena budget (arena_budget.go)
// and may be smaller than requested when the budget is short. An allocation
// that does not fit falls back to the heap; the arena records how many ints it
// could not serve, so the owner can size the next buffer from the real demand.
//
// IMPORTANT: PostfixArena is NOT goroutine-safe. It is intended to be owned
// by a single Csp2dProblemImpl (which itself is owned by a single
// SimulationJob running in its own goroutine).
type PostfixArena struct {
	buf          []int
	cursor       int
	overflowInts int    // ints served from the heap since the last Reset
	overflow     uint64 // overflowing allocations since creation (for logging)
	name         string
}

// NewPostfixArena creates an arena with up to capacityInts ints, as much as the
// arena budget grants.
func NewPostfixArena(name string, capacityInts int) *PostfixArena {
	granted := reserveArenaInts(capacityInts)
	return &PostfixArena{
		buf:  make([]int, granted),
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
		a.overflowInts += n
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

// Demand returns how many ints were requested since the last Reset: those
// served by the arena plus those that fell back to the heap.
func (a *PostfixArena) Demand() int {
	return a.cursor + a.overflowInts
}

func (a *PostfixArena) Reset() {
	// Zero out for safety: any stale slices that someone forgot will read zeros
	// instead of stale data, surfacing bugs faster.
	for i := 0; i < a.cursor; i++ {
		a.buf[i] = 0
	}
	a.cursor = 0
	a.overflowInts = 0
}

// Release returns the buffer to the arena budget. Slices allocated earlier keep
// the memory alive until they become unreachable; the arena itself is empty
// (every later allocation falls back to the heap).
func (a *PostfixArena) Release() {
	freeArenaInts(len(a.buf))
	a.buf = nil
	a.cursor = 0
	a.overflowInts = 0
}

func (a *PostfixArena) Stats() (used, capacity int, overflow uint64) {
	return a.cursor, len(a.buf), atomic.LoadUint64(&a.overflow)
}

// Contains reports whether s was allocated from this arena's current buffer.
// (Slices served from the heap on overflow are not.) The Go heap does not move
// objects, so comparing addresses is sound.
func (a *PostfixArena) Contains(s []int) bool {
	if cap(s) == 0 || len(a.buf) == 0 {
		return false
	}
	base := uintptr(unsafe.Pointer(unsafe.SliceData(a.buf)))
	at := uintptr(unsafe.Pointer(unsafe.SliceData(s)))
	return at >= base && at < base+uintptr(len(a.buf))*unsafe.Sizeof(a.buf[0])
}

func (a *PostfixArena) Capacity() int {
	return len(a.buf)
}

func (a *PostfixArena) OverflowCount() uint64 {
	return atomic.LoadUint64(&a.overflow)
}
