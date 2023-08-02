/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Austin Zhai
 * All rights reserved.
 */
package rproxy

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/jumboframes/armorigo/log"
)

type OptionRProxy func(rproxy *RProxy) error

// You can return a custom data for later usage
type PostAccept func(src net.Addr, dst net.Addr) (interface{}, error)
type PreWrite func(writer io.Writer, custom interface{}) error
type PreDial func(custom interface{}) error
type PostDial func(custom interface{}) error
type Dial func(dst net.Addr, custom interface{}) (net.Conn, error)

func OptionRProxyPostAccept(postAccept PostAccept) OptionRProxy {
	return func(rproxy *RProxy) error {
		rproxy.postAccept = postAccept
		return nil
	}
}

func OptionRProxyPreWrite(preWrite PreWrite) OptionRProxy {
	return func(rproxy *RProxy) error {
		rproxy.preWrite = preWrite
		return nil
	}
}

func OptionRProxyPreDial(preDial PreDial) OptionRProxy {
	return func(rproxy *RProxy) error {
		rproxy.preDial = preDial
		return nil
	}
}

func OptionRProxyPostDial(postDial PostDial) OptionRProxy {
	return func(rproxy *RProxy) error {
		rproxy.postDial = postDial
		return nil
	}
}

func OptionRProxyDial(dial Dial) OptionRProxy {
	return func(rproxy *RProxy) error {
		rproxy.dial = dial
		return nil
	}
}

type RProxy struct {
	listener net.Listener

	//hooks
	postAccept PostAccept
	preWrite   PreWrite
	preDial    PreDial
	postDial   PostDial
	dial       Dial

	//holder
	pipes map[string]*Pipe
	mutex sync.RWMutex
}

func NewRProxy(ln net.Listener, options ...OptionRProxy) (*RProxy, error) {

	rproxy := &RProxy{listener: ln}
	for _, option := range options {
		if err := option(rproxy); err != nil {
			return nil, err
		}
	}

	return rproxy, nil
}

func (rproxy *RProxy) Proxy(ctx context.Context) {
	go func() {
		<-ctx.Done()
		rproxy.listener.Close()
	}()

	for {
		conn, err := rproxy.listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "too many open files") {
				log.Errorf("rproxy accept conn err: %s, retrying", err)
				continue
			}
			log.Debugf("rproxy accept conn err: %s, quiting", err)
			return
		}

		var custom interface{}
		if rproxy.postAccept != nil {
			if custom, err = rproxy.postAccept(conn.RemoteAddr(), conn.LocalAddr()); err != nil {
				log.Errorf("post accept return err: %s", err)
				if err = conn.Close(); err != nil {
					log.Errorf("close left conn err: %s", err)
				}
				continue
			}
		}

		pipe := &Pipe{
			rproxy:   rproxy,
			Src:      conn.RemoteAddr(),
			Dst:      conn.LocalAddr(),
			custom:   custom,
			leftConn: conn,
		}

		go pipe.proxy(ctx)
	}
}

func (rproxy *RProxy) addPipe(pipe *Pipe) {
	rproxy.mutex.Lock()
	key := pipe.Src.String() + pipe.Dst.String()
	rproxy.pipes[key] = pipe
	rproxy.mutex.Unlock()
}

func (rproxy *RProxy) delPipe(pipe *Pipe) {
	rproxy.mutex.Lock()
	key := pipe.Src.String() + pipe.Dst.String()
	delete(rproxy.pipes, key)
	rproxy.mutex.Unlock()
}

func (rproxy *RProxy) Close() {
	if rproxy.listener != nil {
		err := rproxy.listener.Close()
		if err != nil {
			log.Errorf("rproxy listener close err: %s, routine continued", err)
		}
	}
}

type Pipe struct {
	//father
	rproxy *RProxy

	Src net.Addr
	Dst net.Addr

	//custom
	custom interface{}

	leftConn  net.Conn
	rightConn net.Conn
}

func (pipe *Pipe) proxy(ctx context.Context) {
	defer pipe.rproxy.delPipe(pipe)

	// 预先连接
	if pipe.rproxy.preDial != nil {
		if err := pipe.rproxy.preDial(pipe.custom); err != nil {
			log.Errorf("pre dial error: %v", err)
			_ = pipe.leftConn.Close()
			return
		}
	}

	var err error
	dial := func(dst net.Addr, custom interface{}) (net.Conn, error) {
		return net.Dial(dst.Network(), dst.String())
	}
	if pipe.rproxy.dial != nil {
		dial = pipe.rproxy.dial
	}
	pipe.rightConn, err = dial(pipe.Dst, pipe.custom)
	if err != nil {
		log.Errorf("dial error: %v", err)
		_ = pipe.leftConn.Close()
		return
	}

	// 连接后
	if pipe.rproxy.postDial != nil {
		if err := pipe.rproxy.postDial(pipe.custom); err != nil {
			log.Errorf("post dial error: %v", err)
			_ = pipe.leftConn.Close()
			_ = pipe.rightConn.Close()
			return
		}
	}

	// 预先写
	if pipe.rproxy.preWrite != nil {
		if err = pipe.rproxy.preWrite(pipe.rightConn, pipe.custom); err != nil {
			_ = pipe.leftConn.Close()
			_ = pipe.rightConn.Close()
			return
		}
	}

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		defer wg.Done()

		_, err := io.Copy(pipe.leftConn, pipe.rightConn)
		if err != nil {
			log.Errorf("read right, src: %s, dst: %s; to left, src: %s, dst: %s err: %s",
				pipe.rightConn.LocalAddr().String(),
				pipe.rightConn.RemoteAddr().String(),
				pipe.leftConn.RemoteAddr().String(),
				pipe.leftConn.LocalAddr().String(),
				err)
		}
		_ = pipe.rightConn.Close()
		_ = pipe.leftConn.Close()
	}()

	go func() {
		defer wg.Done()

		_, err := io.Copy(pipe.rightConn, pipe.leftConn)
		if err != nil {
			log.Errorf("read left, src: %s, dst: %s; to right, src: %s, dst: %s err: %s",
				pipe.leftConn.RemoteAddr().String(),
				pipe.leftConn.LocalAddr().String(),
				pipe.rightConn.LocalAddr().String(),
				pipe.rightConn.RemoteAddr().String(),
				err)
		}
		_ = pipe.rightConn.Close()
		_ = pipe.leftConn.Close()
	}()

	exist := make(chan struct{})
	defer close(exist)

	go func() {
		select {
		case <-exist:
		case <-ctx.Done():
			log.Debug("force pipe done from parent")
			_ = pipe.leftConn.Close()
			_ = pipe.rightConn.Close()
		}
	}()
	wg.Wait()

	log.Debugf("close right and left conn")
}
