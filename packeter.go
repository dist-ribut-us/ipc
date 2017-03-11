package ipc

import (
	"crypto/rand"
	"github.com/dist-ribut-us/errors"
	"github.com/dist-ribut-us/log"
	"github.com/dist-ribut-us/rnet"
	"github.com/dist-ribut-us/serial"
	"math"
)

// Packeter handles making and collecting packets for inter-process
// communicaiton
type Packeter struct {
	packets map[uint32]*Message
	ch      chan *Message
}

// Chan returns the channel messages will be sent on
func (i *Packeter) Chan() <-chan *Message {
	return i.ch
}

// Receive takes a packet and and address. The address must have an IP of
// 127.0.0.1. All packets in a message must come from the same Port.
func (i *Packeter) Receive(b []byte, addr *rnet.Addr) {
	if addr.IP.String() != "127.0.0.1" {
		log.Info(log.Lbl("non_local_ipc_message"), addr)
		return
	}

	id := serial.UnmarshalUint32(b)
	msg, ok := i.packets[id]
	if !ok {
		msg = &Message{
			ID:   id,
			Len:  int(serial.UnmarshalUint32(b[4:])),
			Addr: addr,
			Body: b[8:],
		}
	} else if addr.Port() == msg.Addr.Port() {
		msg.Body = append(msg.Body, b[4:]...)
	} else {
		log.Info(log.Lbl("message_changed_ports"), log.KV{"started_on", msg.Addr}, log.KV{"now_on", addr})
		return
	}

	if len(msg.Body) >= msg.Len {
		i.ch <- msg
		delete(i.packets, id)
	} else {
		i.packets[id] = msg
	}
}

// Make takes a message, generates a random ID and calls MakeWithID
func (i *Packeter) Make(msg []byte) [][]byte {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	if log.Error(errors.Wrap("generating_packet_id", err)) {
		return nil
	}
	id := (uint32(b[0]) + uint32(b[1])<<8 + uint32(b[2])<<16 + uint32(b[3])<<24)
	return i.MakeWithID(id, msg)
}

// MakeWithID takes an ID and a message and divides it into packets, where each
// is no longer than PacketSize. The message is prepended with the total length
// and each packet is prepended with the ID. There is no mechanism for ordering
// or packet loss, the assumption is that between processes neither will be an
// issue.
func (i *Packeter) MakeWithID(id uint32, msg []byte) [][]byte {
	l := len(msg)
	b := make([]byte, l+4)
	serial.MarshalUint32(uint32(l), b)
	copy(b[4:], msg)

	p := PacketSize - 4
	ln := int(math.Ceil(float64(l) / float64(p)))
	pkts := make([][]byte, ln)
	n := 0
	ids := make([]byte, 4)
	serial.MarshalUint32(id, ids)
	for ; n < ln-1; n++ {
		pkts[n] = make([]byte, PacketSize)
		copy(pkts[n], ids)
		copy(pkts[n][4:], b[n*p:(n+1)*p])
	}
	final := b[n*p:]
	pkts[n] = make([]byte, len(final)+4)
	copy(pkts[n], ids)
	copy(pkts[n][4:], final)

	return pkts
}
