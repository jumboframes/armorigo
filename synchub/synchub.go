package synchub

import (
	"errors"
	"sync"
	"time"

	"github.com/singchia/go-timer/v2"
)

var (
	ErrSyncTimeout        = errors.New("timeout")
	ErrSyncHubForceClosed = errors.New("force closed")
	ErrSyncHubClosed      = errors.New("synchub closed")
	ErrSyncCanceled       = errors.New("sync canceled")
	ErrSyncIDResynced     = errors.New("id resynced")
	ErrTimerNotWorking    = errors.New("timer not working")
)

// A Sync presents the handler of a synchronize.
type Sync interface {
	C() <-chan *Event
	Cancel(notify bool) bool
	Done() bool
	Ack(ack interface{}) bool
	Error(err error) bool
}

type Event struct {
	SyncID, Data, Ack interface{}
	Error             error
}

type synchronize struct {
	sh           *SyncHub
	syncID, data interface{}

	// sync method
	cb func(*Event)
	ch chan *Event

	timeout time.Duration
	tick    timer.Tick
}

func (ec *synchronize) C() <-chan *Event {
	return ec.ch
}

func (ec *synchronize) Cancel(notify bool) bool {
	return ec.sh.Cancel(ec.syncID, notify)
}

func (ec *synchronize) Done() bool {
	return ec.sh.Done(ec.syncID)
}

func (ec *synchronize) Ack(ack interface{}) bool {
	return ec.sh.Ack(ec.syncID, ack)
}

func (ec *synchronize) Error(err error) bool {
	return true
}

// Options for Sync
type SyncOption func(*synchronize)

func WithData(data interface{}) SyncOption {
	return func(sync *synchronize) {
		sync.data = data
	}
}

func WithTimeout(d time.Duration) SyncOption {
	return func(sync *synchronize) {
		sync.timeout = d
	}
}

func WithEventChan(ch chan *Event) SyncOption {
	return func(sync *synchronize) {
		sync.ch = ch
	}
}

// Callback if perfered than channel
func WithCallback(cb func(*Event)) SyncOption {
	return func(sync *synchronize) {
		sync.cb = cb
	}
}

const (
	statusWorking = iota
	statusClosed
)

type SyncHub struct {
	syncs sync.Map

	// status
	status int
	mtx    sync.RWMutex

	// timer
	tmr        timer.Timer
	tmrOutside bool
}

type SyncHubOption func(*SyncHub)

func OptionTimer(tmr timer.Timer) SyncHubOption {
	return func(sh *SyncHub) {
		sh.tmr = tmr
		sh.tmrOutside = true
	}
}

func NewSyncHub(opts ...SyncHubOption) *SyncHub {
	sh := &SyncHub{
		syncs: sync.Map{},
	}
	for _, opt := range opts {
		opt(sh)
	}
	if !sh.tmrOutside {
		sh.tmr = timer.NewTimer()
	}
	return sh
}

func (sh *SyncHub) New(syncID interface{}, opts ...SyncOption) Sync {
	sync := &synchronize{
		sh:     sh,
		syncID: syncID,
	}
	for _, opt := range opts {
		opt(sync)
	}
	if sync.cb == nil && sync.ch == nil {
		sync.ch = make(chan *Event, 1)
	}

	sh.mtx.RLock()
	defer sh.mtx.RUnlock()
	if sh.status == statusClosed {
		event := &Event{
			SyncID: syncID,
			Data:   sync.data,
			Error:  ErrSyncHubClosed,
		}
		if sync.cb != nil {
			sync.cb(event)
			return sync
		}
		sync.ch <- event
		return sync
	}

	for {
		value, loaded := sh.syncs.LoadOrStore(syncID, sync)
		if !loaded {
			break
		}
		event := &Event{
			SyncID: syncID,
			Data:   sync.data,
			Error:  ErrSyncIDResynced,
		}
		old := value.(*synchronize)
		if old.tick != nil {
			old.tick.Cancel()
		}
		if old.cb != nil {
			old.cb(event)
			continue
		}
		old.ch <- event
	}
	if sync.timeout != 0 {
		sync.tick = sh.tmr.Add(sync.timeout, timer.WithData(syncID),
			timer.WithHandler(sh.timeout))
	}
	return sync
}

func (sh *SyncHub) Done(syncID interface{}) bool {
	value, ok := sh.syncs.LoadAndDelete(syncID)
	if !ok {
		return false
	}
	sync := value.(*synchronize)
	if sync.tick != nil {
		sync.tick.Cancel()
	}
	event := &Event{
		SyncID: syncID,
		Data:   sync.data,
	}
	if sync.cb != nil {
		sync.cb(event)
		return true
	}
	sync.ch <- event
	return true
}

func (sh *SyncHub) Ack(syncID interface{}, ack interface{}) bool {
	value, ok := sh.syncs.LoadAndDelete(syncID)
	if !ok {
		return false
	}
	sync := value.(*synchronize)
	if sync.tick != nil {
		sync.tick.Cancel()
	}
	event := &Event{
		SyncID: syncID,
		Data:   sync.data,
		Ack:    ack,
	}
	if sync.cb != nil {
		sync.cb(event)
		return true
	}
	sync.ch <- event
	return true
}

func (sh *SyncHub) Error(syncID interface{}, err error) bool {
	value, ok := sh.syncs.LoadAndDelete(syncID)
	if !ok {
		return false
	}
	sync := value.(*synchronize)
	if sync.tick != nil {
		sync.tick.Cancel()
	}
	event := &Event{
		SyncID: sync.syncID,
		Data:   sync.data,
		Error:  err,
	}
	if sync.cb != nil {
		sync.cb(event)
		return true
	}
	sync.ch <- event
	return true
}

func (sh *SyncHub) Cancel(syncID interface{}, notify bool) bool {
	value, ok := sh.syncs.LoadAndDelete(syncID)
	if !ok {
		return false
	}
	sync := value.(*synchronize)
	if sync.tick != nil {
		sync.tick.Cancel()
	}
	if notify {
		event := &Event{
			SyncID: syncID,
			Data:   sync.data,
			Error:  ErrSyncCanceled,
		}
		if sync.cb != nil {
			sync.cb(event)
			return true
		}
		sync.ch <- event
	}
	return true
}

func (sh *SyncHub) Close() {
	sh.mtx.Lock()
	defer sh.mtx.Unlock()
	sh.status = statusClosed

	sh.syncs.Range(func(key, value interface{}) bool {
		sh.syncs.Delete(key)
		sync := value.(*synchronize)
		if sync.tick != nil {
			sync.tick.Cancel()
		}
		event := &Event{
			SyncID: sync.syncID,
			Data:   sync.data,
			Error:  ErrSyncHubForceClosed,
		}
		if sync.cb != nil {
			sync.cb(event)
			return true
		}
		sync.ch <- event
		return true
	})

	if !sh.tmrOutside {
		sh.tmr.Close()
	}
	sh.tmr = nil
}

func (sh *SyncHub) timeout(tevent *timer.Event) {
	value, ok := sh.syncs.LoadAndDelete(tevent.Data)
	if !ok {
		return
	}
	sync := value.(*synchronize)
	event := &Event{
		SyncID: sync.syncID,
		Data:   sync.data,
		Error:  ErrSyncTimeout,
	}
	if sync.cb != nil {
		sync.cb(event)
		return
	}
	sync.ch <- event
	return
}
