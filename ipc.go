// Package ipc handles inter-process communication using UDP. It limits
// communication to 127.0.0.1 and does not handle packet order or dropped
// packets because these are unlikely issues locally.
package ipc

import (
	"fmt"
	"github.com/dist-ribut-us/errors"
	"github.com/dist-ribut-us/log"
	"github.com/dist-ribut-us/rnet"
)

// Proc represents a process that can send and receive communication from other
// local Procs over UDP.
type Proc struct {
	srv  *rnet.Server
	pktr *packeter
}

// New returns a Proc for sending and receiving communications with other
// local processes. The server will not be running initially.
func New(port rnet.Port) (*Proc, error) {
	var err error
	proc := &Proc{}
	proc.pktr = newPacketer(proc)
	proc.srv, err = rnet.New(port, proc.pktr)
	if err != nil {
		return nil, err
	}

	return proc, nil
}

// Run will start the listen loop. Calling run multiple times will not start
// multiple listen loop.
func (p *Proc) Run() { p.srv.Run() }

// IsRunning indicates if the listen loop is running
func (p *Proc) IsRunning() bool { return p.srv.IsRunning() }

// GetPort returns the UDP port
func (p *Proc) GetPort() rnet.Port { return p.srv.GetPort() }

// String returns the address of the process
func (p *Proc) String() string { return fmt.Sprintf("127.0.0.1:%d", p.srv.GetPort()) }

// IsOpen returns true if the connection is open. If the server is closed, it
// can neither send nor receive
func (p *Proc) IsOpen() bool { return p.srv.IsOpen() }

// Stop will stop the server
func (p *Proc) Stop() error { return p.srv.Stop() }

// Close will close the connection, freeing the port
func (p *Proc) Close() error { return p.srv.Close() }

// Handler adds a handler that will be used instead of the return channel
func (p *Proc) Handler(handler func(*Package)) { p.pktr.handler = handler }

// RunNew returns a Proc for sending and receiving communications with other
// local processes. The server will be running initially.
func RunNew(port rnet.Port) (*Proc, error) {
	p, err := New(port)
	if err != nil {
		return nil, err
	}
	go p.Run()
	return p, nil
}

// Send takes an id, a message and the port of the receiving process and
// sends the message to the other process. It prepends the length of the
// messsage. Unlike the packeter, this does not worry about dropped packets or
// ordering.
func (p *Proc) Send(id uint32, msg []byte, port rnet.Port) {
	if msg == nil {
		return
	}
	pkts := p.pktr.make(id, msg)
	addr := port.On("127.0.0.1")
	if log.Error(errors.Wrap("generating_local_addr_for_ipc", addr.Err)) {
		return
	}
	errs := p.srv.SendAll(pkts, addr)
	if errs != nil {
		log.Info(log.Lbl("while_sending_over_ipc"), errs)
	}
}
