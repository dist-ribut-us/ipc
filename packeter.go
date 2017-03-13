package ipc

import (
	"crypto/rand"
	"github.com/dist-ribut-us/errors"
	"github.com/dist-ribut-us/log"
	"github.com/dist-ribut-us/rnet"
	"github.com/dist-ribut-us/serial"
	"math"
	"sync"
	"time"
)

// packeter handles making and collecting packets for inter-process
// communicaiton
type packeter struct {
	packets   map[uint32]*Message
	ch        chan *Message
	callbacks map[uint32]Callback
	mux       *sync.RWMutex
	proc      *Proc
}

func newPacketer(proc *Proc) *packeter {
	return &packeter{
		packets:   make(map[uint32]*Message),
		ch:        make(chan *Message),
		callbacks: make(map[uint32]Callback),
		mux:       &sync.RWMutex{},
		proc:      proc,
	}
}

// Chan returns the channel messages will be sent on
func (i *packeter) Chan() <-chan *Message {
	return i.ch
}

func (i *packeter) SetCallback(id uint32, callback Callback) {
	i.mux.RLock()
	i.callbacks[id] = callback
	i.mux.RUnlock()
	go i.cleanupCallback(id)
}

func (i *packeter) cleanupCallback(id uint32) {
	time.Sleep(time.Millisecond * 10)
	i.mux.RLock()
	_, timedout := i.callbacks[id]
	i.mux.RUnlock()
	if timedout {
		i.mux.Lock()
		delete(i.callbacks, id)
		i.mux.Unlock()
		log.Info(log.Lbl("callback_timedout"), id)
	}
}

// Receive takes a packet and and address. The address must have an IP of
// 127.0.0.1. All packets in a message must come from the same Port.
func (i *packeter) Receive(b []byte, addr *rnet.Addr) {
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
			proc: i.proc,
		}
	} else if addr.Port() == msg.Addr.Port() {
		msg.Body = append(msg.Body, b[4:]...)
	} else {
		log.Info(log.Lbl("message_changed_ports"), log.KV{"started_on", msg.Addr}, log.KV{"now_on", addr})
		return
	}

	if len(msg.Body) >= msg.Len {
		i.mux.RLock()
		callback, ok := i.callbacks[msg.ID]
		i.mux.RUnlock()
		if ok {
			i.mux.Lock()
			delete(i.callbacks, id)
			i.mux.Unlock()
			b, err := msg.ToBase()
			if !log.Error(err) {
				go callback(b)
			}
		} else {
			i.ch <- msg
		}
		delete(i.packets, id)
	} else {
		i.packets[id] = msg
	}
}

// Make takes a message, generates a random ID and calls MakeWithID
func (i *packeter) Make(msg []byte) [][]byte {
	return i.MakeWithID(randID(), msg)
}

func randID() uint32 {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	if log.Error(errors.Wrap("generating_packet_id", err)) {
		return 0
	}
	return (uint32(b[0]) + uint32(b[1])<<8 + uint32(b[2])<<16 + uint32(b[3])<<24)
}

// MakeWithID takes an ID and a message and divides it into packets, where each
// is no longer than PacketSize. The message is prepended with the total length
// and each packet is prepended with the ID. There is no mechanism for ordering
// or packet loss, the assumption is that between processes neither will be an
// issue.
func (i *packeter) MakeWithID(id uint32, msg []byte) [][]byte {
	l := len(msg)
	b := make([]byte, l+4)
	serial.MarshalUint32(uint32(l), b)
	copy(b[4:], msg)

	p := PacketSize - 4
	ln := int(math.Ceil(float64(l) / float64(p)))
	pkts := make([][]byte, ln)
	n := 0
	ids := serial.MarshalUint32(id, nil)
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
