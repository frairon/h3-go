package h3

/*
#cgo CFLAGS: -std=c99
#cgo CFLAGS: -DH3_HAVE_VLA=1
#cgo LDFLAGS: -lm
#include <stdlib.h>
#include <h3_h3api.h>
#include <h3_h3Index.h>
*/
import "C"

import "sync"

// The PoolDiskCreator is very similar of the "normal"
// GridDiskDistances function, but it uses only pooled values.
// Therefore it holds three maps of pools, each providing different slices.
// Assuming the requested disk sizes are limited and discrete, it creates the
// pools once and resuses them from then on
type PooledDiskCreator struct {
	mPools    sync.RWMutex
	hexPools  map[int]*sync.Pool
	distPools map[int]*sync.Pool
	ringPools map[int]*sync.Pool
}

func NewPoolDiskCreator() *PooledDiskCreator {
	return &PooledDiskCreator{
		hexPools:  make(map[int]*sync.Pool),
		distPools: make(map[int]*sync.Pool),
		ringPools: make(map[int]*sync.Pool),
	}
}

func (pdc *PooledDiskCreator) getPools(radius int) (hexPool, distPool, ringPool *sync.Pool) {
	pdc.mPools.RLock()

	var hexOk, distOk, ringOk bool

	// try to get all pools
	hexPool, hexOk = pdc.hexPools[radius]
	distPool, distOk = pdc.distPools[radius]
	ringPool, ringOk = pdc.ringPools[radius]

	// if one of them doesn't exist
	if hexOk && distOk && ringOk {
		pdc.mPools.RUnlock()
		return hexPool, distPool, ringPool
	}

	// re-lock with write
	pdc.mPools.RUnlock()
	pdc.mPools.Lock()
	// There is a race condition between the RUnlock and the Lock, so we have to check again if
	// another goroutine might have created the pools in the meantime
	hexPool, hexOk = pdc.hexPools[radius]
	distPool, distOk = pdc.distPools[radius]
	ringPool, ringOk = pdc.ringPools[radius]

	// the size used for hex/dist
	size := maxGridDiskSize(radius)

	// create hex pool if not present yet
	if !hexOk {
		hexPool = &sync.Pool{
			New: func() interface{} {
				return make([]C.H3Index, size)
			},
		}
		pdc.hexPools[radius] = hexPool
	}
	// create dist pool if not present yet
	if !distOk {
		distPool = &sync.Pool{
			New: func() interface{} {
				return make([]C.int, size)
			},
		}
		pdc.distPools[radius] = distPool
	}

	// create ring pool if not present yet
	if !ringOk {
		ringPool = &sync.Pool{
			New: func() interface{} {
				ret := make([][]Cell, radius+1)
				for i := 0; i <= radius; i++ {
					ret[i] = make([]Cell, 0, ringSize(i))
				}
				return ret
			},
		}
		pdc.ringPools[radius] = ringPool
	}

	pdc.mPools.Unlock()

	return hexPool, distPool, ringPool
}

// Returns a pooled disk grid.
// The caller must ensure to call returned closer function - or slice will be leaked
func (pdc *PooledDiskCreator) GridDiskDistances(origin Cell, k int) ([][]Cell, func()) {
	hexPool, distPool, ringPool := pdc.getPools(k)

	outHexes := hexPool.Get().([]C.H3Index)
	outDists := distPool.Get().([]C.int)
	defer hexPool.Put(outHexes)
	defer distPool.Put(outDists)

	// get the cells/distances
	C.gridDiskDistances(C.H3Index(origin), C.int(k), &outHexes[0], &outDists[0])

	// get a ring from the pool and fill it
	ret := ringPool.Get().([][]Cell)
	for i, d := range outDists {
		ret[d] = append(ret[d], Cell(outHexes[i]))
		// reset the slices for next reusage
		outHexes[i] = 0
		outDists[i] = 0
	}

	return ret, func() {
		// reset length of slices
		for i := range ret {
			ret[i] = ret[i][0:0]
		}
		ringPool.Put(ret)
	}
}
