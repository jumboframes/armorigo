package rproxy

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type countedListener struct {
	net.Listener
	closes int32
	err    error
}

func (l *countedListener) Close() error {
	atomic.AddInt32(&l.closes, 1)
	if l.Listener != nil {
		return l.Listener.Close()
	}
	return l.err
}

func TestCloseListenerConcurrent(t *testing.T) {
	for _, failure := range []error{nil, net.ErrClosed, errors.New("close failure")} {
		l := &countedListener{err: failure}
		p, err := NewRProxy(l)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for i := 0; i < 32; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := p.CloseListener()
				want := failure
				if errors.Is(want, net.ErrClosed) {
					want = nil
				}
				if !errors.Is(err, want) {
					t.Errorf("close: got %v want %v", err, want)
				}
			}()
		}
		wg.Wait()
		p.Close()
		if got := atomic.LoadInt32(&l.closes); got != 1 {
			t.Fatalf("closed %d times", got)
		}
	}
	var zero RProxy
	if err := zero.CloseListener(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseListenerPreservesEstablishedConnection(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "CloseListener"
		if legacy {
			name = "Close"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			backend, target := net.Pipe()
			defer backend.Close()
			defer target.Close()
			p, err := NewRProxy(listener, OptionRProxyDial(func(_ net.Addr, _ interface{}) (net.Conn, error) { return backend, nil }))
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			done := make(chan struct{})
			go func() { defer close(done); p.Proxy(ctx) }()
			client, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if err := client.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if err := target.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			exchange := func() {
				t.Helper()
				if _, err := client.Write([]byte("a")); err != nil {
					t.Fatal(err)
				}
				data := make([]byte, 1)
				if _, err := io.ReadFull(target, data); err != nil || data[0] != 'a' {
					t.Fatalf("upstream: %q %v", data, err)
				}
				if _, err := target.Write([]byte("b")); err != nil {
					t.Fatal(err)
				}
				if _, err := io.ReadFull(client, data); err != nil || data[0] != 'b' {
					t.Fatalf("downstream: %q %v", data, err)
				}
			}
			exchange()
			if legacy {
				p.Close()
			} else if err := p.CloseListener(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("Accept loop did not exit")
			}
			other, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
			if err == nil {
				other.Close()
				t.Fatal("closed listener accepted new TCP connection")
			}
			exchange()
			cancel()
			data := make([]byte, 1)
			if _, err := client.Read(data); err == nil {
				t.Fatal("context cancellation did not close pipe")
			} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				t.Fatal("pipe only stopped because test timed out")
			}
		})
	}
}

func TestCloseListenerBeforeProxy(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewRProxy(l)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.CloseListener(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); p.Proxy(ctx) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("closed listener loops")
	}
}
