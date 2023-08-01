/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Austin Zhai
 * All rights reserved.
 */
package pproxy

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/jumboframes/armorigo/log"
)

type OptionPProxy func(pproxy *PProxy) error

// You can return a custom data for later usage
type PostAccept func(src net.Addr, dst net.Addr) (interface{}, error)
type PreWrite func(writer io.Writer, custom interface{}) error
type PreDial func(custom interface{}) error
type PostDial func(custom interface{}) error
type Dial func(dst net.Addr, custom interface{}) (net.Conn, error)

func OptionPProxyPostAccept(postAccept PostAccept) OptionPProxy {
	return func(pproxy *PProxy) error {
		pproxy.postAccept = postAccept
		return nil
	}
}

func OptionPProxyPreWrite(preWrite PreWrite) OptionPProxy {
	return func(pproxy *PProxy) error {
		pproxy.preWrite = preWrite
		return nil
	}
}

func OptionPProxyPreDial(preDial PreDial) OptionPProxy {
	return func(pproxy *PProxy) error {
		pproxy.preDial = preDial
		return nil
	}
}

func OptionPProxyPostDial(postDial PostDial) OptionPProxy {
	return func(pproxy *PProxy) error {
		pproxy.postDial = postDial
		return nil
	}
}

func OptionPProxyDial(dial Dial) OptionPProxy {
	return func(pproxy *PProxy) error {
		pproxy.dial = dial
		return nil
	}
}

type PProxy struct {
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

func NewPProxy(ln net.Listener, options ...OptionPProxy) (*PProxy, error) {

	pproxy := &PProxy{listener: ln}
	for _, option := range options {
		if err := option(pproxy); err != nil {
			return nil, err
		}
	}

	return pproxy, nil
}

func (pproxy *PProxy) Proxy(ctx context.Context) {
	go func() {
		<-ctx.Done()
		pproxy.listener.Close()
	}()

	for {
		conn, err := pproxy.listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "too many open files") {
				log.Errorf("pproxy accept conn err: %s, retrying", err)
				continue
			}
			log.Debugf("pproxy accept conn err: %s, quiting", err)
			return
		}

		var custom interface{}
		if pproxy.postAccept != nil {
			if custom, err = pproxy.postAccept(conn.RemoteAddr(), conn.LocalAddr()); err != nil {
				log.Errorf("post accept return err: %s", err)
				if err = conn.Close(); err != nil {
					log.Errorf("close left conn err: %s", err)
				}
				continue
			}
		}

		pipe := &Pipe{
			pproxy:   pproxy,
			Src:      conn.RemoteAddr(),
			Dst:      conn.LocalAddr(),
			custom:   custom,
			leftConn: conn,
		}

		go pipe.proxy(ctx)
	}
}

func (pproxy *PProxy) addPipe(pipe *Pipe) {
	pproxy.mutex.Lock()
	key := pipe.Src.String() + pipe.Dst.String()
	pproxy.pipes[key] = pipe
	pproxy.mutex.Unlock()
}

func (pproxy *PProxy) delPipe(pipe *Pipe) {
	pproxy.mutex.Lock()
	key := pipe.Src.String() + pipe.Dst.String()
	delete(pproxy.pipes, key)
	pproxy.mutex.Unlock()
}

func (pproxy *PProxy) Close() {
	if pproxy.listener != nil {
		err := pproxy.listener.Close()
		if err != nil {
			log.Errorf("pproxy listener close err: %s, routine continued", err)
		}
	}
}

type Pipe struct {
	//father
	pproxy *PProxy

	Src net.Addr
	Dst net.Addr

	//custom
	custom interface{}

	leftConn  net.Conn
	rightConn net.Conn
}

func (pipe *Pipe) proxy(ctx context.Context) {
	defer pipe.pproxy.delPipe(pipe)

	// 预先连接
	if pipe.pproxy.preDial != nil {
		if err := pipe.pproxy.preDial(pipe.custom); err != nil {
			log.Errorf("pre dial error: %v", err)
			_ = pipe.leftConn.Close()
			return
		}
	}

	var err error
	dial := func(dst net.Addr, custom interface{}) (net.Conn, error) {
		return net.Dial(dst.Network(), dst.String())
	}
	if pipe.pproxy.dial != nil {
		dial = pipe.pproxy.dial
	}
	pipe.rightConn, err = dial(pipe.Dst, pipe.custom)
	if err != nil {
		log.Errorf("dial error: %v", err)
		_ = pipe.leftConn.Close()
		return
	}

	// 连接后
	if pipe.pproxy.postDial != nil {
		if err := pipe.pproxy.postDial(pipe.custom); err != nil {
			log.Errorf("post dial error: %v", err)
			_ = pipe.leftConn.Close()
			_ = pipe.rightConn.Close()
			return
		}
	}

	// 预先写
	if pipe.pproxy.preWrite != nil {
		if err = pipe.pproxy.preWrite(pipe.rightConn, pipe.custom); err != nil {
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
