/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Austin Zhai
 * All rights reserved.
 */
package sigaction

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestSignal(t *testing.T) {
	signal.Reset()
	foo := &foo{}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		fmt.Println("sub routine quit")
	}()

	go func() {
		t := time.NewTimer(time.Second)
		<-t.C
		pid := os.Getpid()
		syscall.Kill(pid, syscall.SIGINT)
	}()

	sig := NewSignal(OptionSignalCancel(cancel))
	sig.Add(syscall.SIGINT, foo)
	sig.Wait(ctx)
}

type foo struct{}

func (foo *foo) Notify(sg os.Signal) {
	fmt.Println("notification: ", sg.String())
}
