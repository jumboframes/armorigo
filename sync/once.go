package sync

import (
	"sync"
	"sync/atomic"
)

type Once struct {
	done uint32
	m    sync.Mutex
}

func (o *Once) Do(f func()) {
	if atomic.LoadUint32(&o.done) == 0 {
		o.doSlow(f)
	}
}

func (o *Once) doSlow(f func()) {
	// Add for waiting lock.
	ok := o.m.TryLock()
	if !ok {
		return
	}
	defer o.m.Unlock()
	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		f()
	}
}
