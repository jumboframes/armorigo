package synchub

import (
	"errors"
	"sync"

	"github.com/jumboframes/armorigo/log"

	"github.com/singchia/go-timer"
)

var (
	ErrTimeout     = errors.New("timeout")
	ErrNotfound    = errors.New("not found")
	ErrForceClosed = errors.New("force closed")
)

type SyncHub struct {
	syncs sync.Map
	tmr   timer.Timer
}

func NewSyncHub(tmr timer.Timer) *SyncHub {
	sh := &SyncHub{
		syncs: sync.Map{},
		tmr:   tmr,
	}
	return sh
}

type Event struct {
	Value interface{}
	Ack   interface{}
	Error error
}

type eventchan struct {
	ch    chan *Event
	event *Event
	tick  timer.Tick
	cb    func(*Event)
}

func (sh *SyncHub) Sync(syncID interface{}, value interface{}, timeout uint64) chan *Event {
	event := &Event{
		Value: value,
		Ack:   nil,
		Error: nil,
	}
	ch := make(chan *Event, 1)
	uNc := &eventchan{ch: ch, event: event}
	tick, err := sh.tmr.Time(timeout, syncID, nil, sh.timeout)
	if err != nil {
		log.Debugf("timer set err: %s", err)
		return nil
	}
	uNc.tick = tick
	value, ok := sh.syncs.Swap(syncID, uNc)
	if ok {
		uNcOld := value.(*eventchan)
		uNcOld.tick.Cancel()
		uNcOld.event.Error = errors.New("the id was resynced")
		uNcOld.ch <- uNcOld.event
	}
	return ch
}

func (sh *SyncHub) Ack(syncID interface{}, ack interface{}) {
	defer sh.syncs.Delete(syncID)

	value, ok := sh.syncs.Load(syncID)
	if !ok {
		return
	}
	uNc := value.(*eventchan)
	uNc.tick.Cancel()
	uNc.event.Ack = ack
	if uNc.ch != nil {
		uNc.ch <- uNc.event
	}
	if uNc.cb != nil {
		uNc.cb(uNc.event)
	}
}

func (sh *SyncHub) Error(syncID interface{}, err error) {
	defer sh.syncs.Delete(syncID)

	value, ok := sh.syncs.Load(syncID)
	if !ok {
		return
	}
	uNc := value.(*eventchan)
	uNc.tick.Cancel()
	uNc.event.Error = err
	if uNc.ch != nil {
		uNc.ch <- uNc.event
	}
	if uNc.cb != nil {
		uNc.cb(uNc.event)
	}
}

func (sh *SyncHub) Cancel(syncID interface{}) {
	value, ok := sh.syncs.Load(syncID)
	if ok {
		uNc := value.(*eventchan)
		uNc.tick.Cancel()
		sh.syncs.Delete(syncID)
		log.Debugf("syncID: %v canceled", syncID)
	}
}

func (sh *SyncHub) Close() {
	sh.syncs.Range(func(key, value interface{}) bool {
		log.Debugf("syncID: %v force closed", key)
		uNc := value.(*eventchan)
		uNc.event.Error = ErrForceClosed
		if uNc.ch != nil {
			uNc.ch <- uNc.event
		}
		if uNc.cb != nil {
			uNc.cb(uNc.event)
		}
		return true
	})
}

func (sh *SyncHub) timeout(data interface{}) error {
	defer sh.syncs.Delete(data)

	value, ok := sh.syncs.Load(data)
	if !ok {
		log.Debugf("syncID: %v not found", data)
		return nil
	}
	uNc := value.(*eventchan)
	uNc.event.Error = ErrTimeout
	if uNc.ch != nil {
		uNc.ch <- uNc.event
	}
	if uNc.cb != nil {
		uNc.cb(uNc.event)
	}
	log.Debugf("syncID: %v timeout", data)
	return nil
}
