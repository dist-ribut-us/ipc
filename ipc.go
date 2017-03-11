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

// PacketSize is the max packet size, it's set a bit less than the absolute max
// at a nice, round value.
var PacketSize = 50000

// Proc represents a process that can send and receive communication from other
// local Procs over UDP.
type Proc struct {
	srv  *rnet.Server
	pktr *Packeter
}

// New returns a Proc for sending and receiving communications with other
// local processes. The server will not be running initially.
func New(port rnet.Port) (*Proc, error) {
	p := &Packeter{
		packets: make(map[uint32]*Message),
		ch:      make(chan *Message),
	}

	srv, err := rnet.New(port, p)
	if err != nil {
		return nil, err
	}

	return &Proc{
		srv:  srv,
		pktr: p,
	}, nil
}

// Run will start the listen loop. Calling run multiple times will not start
// multiple listen loop.
func (p *Proc) Run() { p.srv.Run() }

// IsRunning indicates if the listen loop is running
func (p *Proc) IsRunning() bool { return p.srv.IsRunning() }

// Port returns the UDP port
func (p *Proc) Port() rnet.Port { return p.srv.Port() }

// String returns the address of the process
func (p *Proc) String() string { return fmt.Sprintf("127.0.0.1:%d", p.srv.Port()) }

// IsOpen returns true if the connection is open. If the server is closed, it
// can neither send nor receive
func (p *Proc) IsOpen() bool { return p.srv.IsOpen() }

// Stop will stop the server
func (p *Proc) Stop() error { return p.srv.Stop() }

// Close will close the connection, freeing the port
func (p *Proc) Close() error { return p.srv.Close() }

// RunNew returns a Proc for sending and receiving communications with other
// local processes. The server will be running initially.
func RunNew(port rnet.Port) (*Proc, error) {
	p := &Packeter{
		packets: make(map[uint32]*Message),
		ch:      make(chan *Message),
	}

	srv, err := rnet.RunNew(port, p)
	if err != nil {
		return nil, err
	}

	return &Proc{
		srv:  srv,
		pktr: p,
	}, nil
}

// Chan returns the channel messages will be sent on from the packeter
func (p *Proc) Chan() <-chan *Message {
	return p.pktr.Chan()
}

// Send takes a message and the port of the receiving process and sends the
// message to the other process. It prepends the length of the messsage. Unlike
// the Packeter, this does not worry about dropped packets or ordering.
func (p *Proc) Send(msg []byte, port rnet.Port) {
	pkts := p.pktr.Make(msg)
	addr := port.On("127.0.0.1")
	if log.Error(errors.Wrap("generating_local_addr_for_ipc", addr.Err)) {
		return
	}
	errs := p.srv.SendAll(pkts, addr)
	if errs != nil {
		log.Info(log.Lbl("while_sending_over_ipc"), errs)
	}
}

// SendResponse takes a response and the the wrapped query it is responding to
// and sends the response with the same message id to source of the query.
func (p *Proc) SendResponse(r *Response, q *Wrapper) {
	msg, err := r.Wrap()
	if log.Error(errors.Wrap("wrapping_response_to_send", err)) {
		return
	}
	pkts := p.pktr.MakeWithID(q.Id, msg)
	addr := q.Port().On("127.0.0.1")
	if log.Error(errors.Wrap("generating_local_addr_for_ipc", addr.Err)) {
		return
	}
	errs := p.srv.SendAll(pkts, addr)
	if errs != nil {
		log.Info(log.Lbl("while_sending_response_over_ipc"), errs)
	}
}
