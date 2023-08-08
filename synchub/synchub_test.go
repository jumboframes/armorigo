package synchub

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSyncHub_New(t *testing.T) {
	sh := NewSyncHub()

	type args struct {
		syncID interface{}
		data   interface{}
		opts   []SyncOption
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{
			name: "1",
			args: args{
				syncID: 1,
				data:   1,
				opts:   []SyncOption{WithData(1)},
			},
		},
		{
			name: "2",
			args: args{
				syncID: 2,
				data:   2,
				opts:   []SyncOption{WithData(2)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sync := sh.New(tt.args.syncID, tt.args.opts...)
			go sh.Done(tt.args.syncID)
			event := <-sync.C()
			assert.Equal(t, tt.args.data, event.Data)
		})
	}
}

func TestSyncHub_Done(t *testing.T) {
	sh := NewSyncHub()

	type args struct {
		syncID interface{}
		data   interface{}
		opts   []SyncOption
	}
	tests := []struct {
		name string
		args args
	}{}
	for i := 0; i < 10000; i++ {
		test := struct {
			name string
			args args
		}{
			name: strconv.Itoa(i),
			args: args{
				syncID: i,
				data:   i,
				opts:   []SyncOption{WithData(i)},
			},
		}
		tests = append(tests, test)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sync := sh.New(tt.args.syncID, tt.args.opts...)
			go sh.Done(tt.args.syncID)
			event := <-sync.C()
			assert.Equal(t, tt.args.data, event.Data)
		})
	}
}

func TestSyncHub_DoneSub(t *testing.T) {
	sh := NewSyncHub()

	type args struct {
		syncID interface{}
		data   interface{}
		opts   []SyncOption
	}
	tests := []struct {
		name string
		args args
	}{}
	for i := 0; i < 10000; i++ {
		test := struct {
			name string
			args args
		}{
			name: strconv.Itoa(i),
			args: args{
				syncID: i,
				data:   i,
				opts:   []SyncOption{WithData(i), WithSub(i, "b")},
			},
		}
		tests = append(tests, test)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sync := sh.Add(tt.args.syncID, tt.args.opts...)
			go sh.DoneSub(tt.args.syncID, "b")
			go sh.DoneSub(tt.args.syncID, tt.args.data)
			event := <-sync.C()
			assert.Equal(t, tt.args.data, event.Data)
		})
	}
}

func TestSyncHub_Ack(t *testing.T) {
	sh := NewSyncHub()

	type args struct {
		syncID interface{}
		data   interface{}
		ack    interface{}
		opts   []SyncOption
	}
	tests := []struct {
		name string
		args args
	}{}
	base := 10000
	for i := 0; i < base; i++ {
		test := struct {
			name string
			args args
		}{
			name: strconv.Itoa(i),
			args: args{
				syncID: i,
				data:   i,
				ack:    base + i,
				opts:   []SyncOption{WithData(i)},
			},
		}
		tests = append(tests, test)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sync := sh.New(tt.args.syncID, tt.args.opts...)
			go sh.Ack(tt.args.syncID, tt.args.data.(int)+base)
			event := <-sync.C()
			assert.Equal(t, tt.args.ack, event.Ack)
		})
	}
}

func TestSyncHub_Error(t *testing.T) {
	sh := NewSyncHub()

	type args struct {
		syncID interface{}
		data   interface{}
		ack    interface{}
		opts   []SyncOption
	}
	tests := []struct {
		name string
		args args
	}{}
	base := 10000
	for i := 0; i < base; i++ {
		test := struct {
			name string
			args args
		}{
			name: strconv.Itoa(i),
			args: args{
				syncID: i,
				data:   i,
				opts:   []SyncOption{WithData(i)},
			},
		}
		tests = append(tests, test)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sync := sh.New(tt.args.syncID, tt.args.opts...)
			err := errors.New(fmt.Sprint(tt.args.data))
			go sh.Error(tt.args.syncID, err)
			event := <-sync.C()
			assert.Equal(t, err, event.Error)
		})
	}
}

func TestSyncHub_Cancel(t *testing.T) {
	sh := NewSyncHub()

	type args struct {
		syncID interface{}
		data   interface{}
		ack    interface{}
		opts   []SyncOption
	}
	tests := []struct {
		name string
		args args
	}{}
	base := 10000
	for i := 0; i < base; i++ {
		test := struct {
			name string
			args args
		}{
			name: strconv.Itoa(i),
			args: args{
				syncID: i,
				data:   i,
				opts:   []SyncOption{WithData(i)},
			},
		}
		tests = append(tests, test)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sync := sh.New(tt.args.syncID, tt.args.opts...)
			err := errors.New(fmt.Sprint(tt.args.data))
			go sh.Error(tt.args.syncID, err)
			event := <-sync.C()
			assert.Equal(t, err, event.Error)
		})
	}
}

func TestSyncHub_Close(t *testing.T) {
	sh := NewSyncHub()

	type args struct {
		syncID interface{}
		data   interface{}
		ack    interface{}
		opts   []SyncOption
	}
	tests := []struct {
		name string
		args args
	}{}
	base := 10000
	for i := 0; i < base; i++ {
		test := struct {
			name string
			args args
		}{
			name: strconv.Itoa(i),
			args: args{
				syncID: i,
				data:   i,
				opts:   []SyncOption{WithData(i)},
			},
		}
		tests = append(tests, test)
	}
	closed := int64(0)
	forceClosed := int64(0)
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for _, tt := range tests {
		go func(tt struct {
			name string
			args args
		}) {
			defer wg.Done()
			sync := sh.New(tt.args.syncID, tt.args.opts...)
			event := <-sync.C()
			if event.Error == nil {
				t.Error("should not be here")
				return
			}
			if event.Error == ErrSyncHubClosed {
				atomic.AddInt64(&closed, 1)
				return
			}
			if event.Error == ErrSyncHubForceClosed {
				atomic.AddInt64(&forceClosed, 1)
				return
			}
			t.Error(event.Error)
		}(tt)
	}
	sh.Close()
	wg.Wait()
	t.Log(closed, forceClosed)
}

func TestSyncHub_NewWithTimeout(t *testing.T) {
	sh := NewSyncHub()

	type args struct {
		syncID interface{}
		data   interface{}
		ack    interface{}
		opts   []SyncOption
	}
	tests := []struct {
		name string
		args args
	}{}
	base := 10000
	for i := 0; i < base; i++ {
		test := struct {
			name string
			args args
		}{
			name: strconv.Itoa(i),
			args: args{
				syncID: i,
				data:   i,
				opts:   []SyncOption{WithData(i), WithTimeout(time.Second)},
			},
		}
		tests = append(tests, test)
	}
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for _, tt := range tests {
		go func(tt struct {
			name string
			args args
		}) {
			defer wg.Done()
			sync := sh.New(tt.args.syncID, tt.args.opts...)
			event := <-sync.C()
			assert.Equal(t, ErrSyncTimeout, event.Error)
		}(tt)
	}
	wg.Wait()
}

func TestSyncHub_NewWithCallback(t *testing.T) {
	sh := NewSyncHub()

	type args struct {
		syncID interface{}
		data   interface{}
		ack    interface{}
		opts   []SyncOption
	}
	tests := []struct {
		name string
		args args
	}{}
	base := 10000
	for i := 0; i < base; i++ {
		test := struct {
			name string
			args args
		}{
			name: strconv.Itoa(i),
			args: args{
				syncID: i,
				data:   i,
				opts: []SyncOption{WithData(i), WithTimeout(time.Second), WithCallback(func(event *Event) {
					assert.Equal(t, ErrSyncTimeout, event.Error)
				})},
			},
		}
		tests = append(tests, test)
	}
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for _, tt := range tests {
		go func(tt struct {
			name string
			args args
		}) {
			defer wg.Done()
			sh.New(tt.args.syncID, tt.args.opts...)
		}(tt)
	}
	wg.Wait()
}
