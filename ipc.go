// Package ipc handles inter-process communication using UDP. It limits
// communication to 127.0.0.1 and does not handle packet order or dropped
// packets because these are unlikely issues locally.
package ipc

import (
	"crypto/rand"
	"fmt"
	"github.com/dist-ribut-us/rnet"
	"github.com/dist-ribut-us/serial"
	"math"
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
func New(port int) (*Proc, error) {
	p := &Packeter{
		packets: make(map[uint32]*Message),
		ch:      make(chan *Message),
	}

	srv, err := rnet.New(fmt.Sprintf("127.0.0.1:%d", port), p)
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
func (p *Proc) Port() int { return p.srv.Port() }

// IsOpen returns true if the connection is open. If the server is closed, it
// can neither send nor receive
func (p *Proc) IsOpen() bool { return p.srv.IsOpen() }

// Stop will stop the server
func (p *Proc) Stop() error { return p.srv.Stop() }

// Close will close the connection, freeing the port
func (p *Proc) Close() error { return p.srv.Close() }

// RunNew returns a Proc for sending and receiving communications with other
// local processes. The server will be running initially.
func RunNew(port int) (*Proc, error) {
	p := &Packeter{
		packets: make(map[uint32]*Message),
		ch:      make(chan *Message),
	}

	srv, err := rnet.RunNew(fmt.Sprintf("127.0.0.1:%d", port), p)
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
func (p *Proc) Send(msg []byte, port int) error {
	pkts, err := p.pktr.Make(msg)
	if err != nil {
		return err
	}

	addr, err := rnet.ResolveAddr(fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}

	err = p.srv.SendAll(pkts, addr)
	return err
}

// Packeter handles making and collecting packets for inter-process
// communicaiton
type Packeter struct {
	packets map[uint32]*Message
	ch      chan *Message
}

// Message is used to assemble the messages and send them through the channel
// when they are complete.
type Message struct {
	ID   uint32
	Body []byte
	Addr *rnet.Addr
	Len  int
}

// Chan returns the channel messages will be sent on
func (i *Packeter) Chan() <-chan *Message {
	return i.ch
}

// Receive takes a packet and and address. The address must have an IP of
// 127.0.0.1. All packets in a message must come from the same Port.
func (i *Packeter) Receive(b []byte, addr *rnet.Addr) {
	if addr.IP.String() != "127.0.0.1" {
		return
	}

	id := serial.UnmarshalUint32(b)
	msg, ok := i.packets[id]
	if !ok {
		msg = &Message{
			ID:   id,
			Body: b[8:],
			Addr: addr,
			Len:  int(serial.UnmarshalUint32(b[4:])),
		}
	} else if addr.Port == msg.Addr.Port {
		msg.Body = append(msg.Body, b[4:]...)
	} else {
		return
	}

	if len(msg.Body) >= msg.Len {
		i.ch <- msg
		delete(i.packets, id)
	} else {
		i.packets[id] = msg
	}
}

// Make takes a message and divides it into packets, where each is no longer
// than PacketSize. The message is prepended with the total length and each
// packet is prepended with an ID. There is no mechanism for ordering or packet
// loss, the assumption is that between processes neither will be an issue.
func (i *Packeter) Make(msg []byte) ([][]byte, error) {
	l := len(msg)
	b := make([]byte, l+4)
	serial.MarshalUint32(uint32(l), b)
	copy(b[4:], msg)

	id := make([]byte, 4)
	_, err := rand.Read(id)
	if err != nil {
		return nil, err
	}

	p := PacketSize - 4
	ln := int(math.Ceil(float64(l) / float64(p)))
	pkts := make([][]byte, ln)
	n := 0
	for ; n < ln-1; n++ {
		pkts[n] = make([]byte, PacketSize)
		copy(pkts[n], id)
		copy(pkts[n][4:], b[n*p:(n+1)*p])
	}
	final := b[n*p:]
	pkts[n] = make([]byte, len(final)+4)
	copy(pkts[n], id)
	copy(pkts[n][4:], final)

	return pkts, nil
}
