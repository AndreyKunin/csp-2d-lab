package csp2d

import "sync/atomic"

// Process-wide accounting of the memory held by postfix arenas (all jobs).
//
// The arenas are only a fast path: an allocation that does not fit falls back
// to the ordinary heap, where the GC collects it. The budget therefore bounds
// how much memory the arenas pin for the lifetime of a job, so the rest of the
// memory limit stays available for heap objects (populations, forests, HTTP
// traffic, overflow allocations) and the Go runtime.
var (
	arenaBudget   atomic.Int64 // bytes; <= 0 means unlimited
	arenaReserved atomic.Int64 // bytes currently held by arena buffers
)

const bytesPerInt = 8

// SetArenaBudget sets the process-wide limit, in bytes, for all postfix arenas
// together. 0 or a negative value means unlimited. Arenas that already exist
// keep their buffers; the limit applies to later reservations.
func SetArenaBudget(bytes int64) { arenaBudget.Store(bytes) }

// ArenaBudget returns the process-wide arena limit in bytes (<= 0: unlimited).
func ArenaBudget() int64 { return arenaBudget.Load() }

// ArenaBytesReserved returns the bytes currently held by arena buffers.
func ArenaBytesReserved() int64 { return arenaReserved.Load() }

// reserveArenaInts reserves up to want ints (and at least nothing) and returns
// how many were granted.
func reserveArenaInts(want int) int {
	if want <= 0 {
		return 0
	}
	for {
		budget := arenaBudget.Load()
		reserved := arenaReserved.Load()
		grant := int64(want)
		if budget > 0 {
			free := (budget - reserved) / bytesPerInt
			if free <= 0 {
				return 0
			}
			grant = min(grant, free)
		}
		if arenaReserved.CompareAndSwap(reserved, reserved+grant*bytesPerInt) {
			return int(grant)
		}
	}
}

// freeArenaInts returns a reservation of n ints.
func freeArenaInts(n int) {
	if n > 0 {
		arenaReserved.Add(-int64(n) * bytesPerInt)
	}
}

// availableArenaInts reports how many more ints could be reserved now
// (math.MaxInt when unlimited).
func availableArenaInts() int {
	budget := arenaBudget.Load()
	if budget <= 0 {
		return int(^uint(0) >> 1)
	}
	free := (budget - arenaReserved.Load()) / bytesPerInt
	return int(max(free, 0))
}
