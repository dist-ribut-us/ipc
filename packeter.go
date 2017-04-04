package ipc

import (
	"github.com/dist-ribut-us/log"
	"github.com/dist-ribut-us/rnet"
	"github.com/dist-ribut-us/serial"
	"math"
	"sync"
	"time"
)

// PacketSize is the max packet size, it's set a bit less than the absolute max
// at a nice, round value.
var PacketSize = 50000

// packeter handles making and collecting packets for inter-process
// communicaiton
type packeter struct {
	packets      map[uint32]*Package
	packetsMux   sync.RWMutex
	ch           chan *Package
	callbacks    map[uint32]Callback
	callbacksMux sync.RWMutex
	proc         *Proc
	handler      func(*Base)
}

func newPacketer(proc *Proc) *packeter {
	return &packeter{
		packets:   make(map[uint32]*Package),
		ch:        make(chan *Package),
		callbacks: make(map[uint32]Callback),
		proc:      proc,
	}
}

func (p *packeter) setCallback(id uint32, callback Callback) {
	p.callbacksMux.RLock()
	p.callbacks[id] = callback
	p.callbacksMux.RUnlock()
	go p.cleanupCallback(id)
}

func (p *packeter) cleanupCallback(id uint32) {
	time.Sleep(time.Millisecond * 20)
	p.callbacksMux.RLock()
	_, timedout := p.callbacks[id]
	p.callbacksMux.RUnlock()
	if timedout {
		p.callbacksMux.Lock()
		delete(p.callbacks, id)
		p.callbacksMux.Unlock()
		log.Info(log.Lbl("callback_timedout"), id)
	}
}

// Receive takes a packet and and address. The address must have an IP of
// 127.0.0.1. All packets in a message must come from the same Port.
func (p *packeter) Receive(b []byte, addr *rnet.Addr) {
	if !(addr.IP == nil || addr.IP.String() == "127.0.0.1") {
		log.Info(log.Lbl("non_local_ipc_message"), addr)
		return
	}

	id := serial.UnmarshalUint32(b)
	p.packetsMux.RLock()
	pkg, ok := p.packets[id]
	p.packetsMux.RUnlock()
	if !ok {
		pkg = &Package{
			ID:   id,
			Len:  int(serial.UnmarshalUint32(b[4:])),
			Addr: addr,
			Body: b[8:],
			proc: p.proc,
		}
	} else if addr.Port() == pkg.Addr.Port() {
		pkg.Body = append(pkg.Body, b[4:]...)
	} else {
		log.Info(log.Lbl("message_changed_ports"), log.KV{"started_on", pkg.Addr}, log.KV{"now_on", addr})
		return
	}

	if len(pkg.Body) >= pkg.Len {
		p.callbacksMux.RLock()
		callback, ok := p.callbacks[pkg.ID]
		p.callbacksMux.RUnlock()
		if ok {
			b, err := pkg.ToBase()
			if !log.Error(err) && b.IsResponse() {
				p.callbacksMux.Lock()
				delete(p.callbacks, id)
				p.callbacksMux.Unlock()
				go callback(b)
				p.packetsMux.Lock()
				delete(p.packets, id)
				p.packetsMux.Unlock()
				return
			}
		}

		if p.handler != nil {
			b, err := pkg.ToBase()
			if !log.Error(err) {
				go p.handler(b)
			}
		} else {
			p.ch <- pkg
		}

		p.packetsMux.Lock()
		delete(p.packets, id)
		p.packetsMux.Unlock()
	} else {
		p.packetsMux.Lock()
		p.packets[id] = pkg
		p.packetsMux.Unlock()
	}
}

// make takes an ID and a message and divides it into packets, where each
// is no longer than PacketSize. The message is prepended with the total length
// and each packet is prepended with the ID. There is no mechanism for ordering
// or packet loss, the assumption is that between processes neither will be an
// issue.
func (p *packeter) make(id uint32, pkg []byte) [][]byte {
	l := len(pkg)
	b := make([]byte, l+4)
	serial.MarshalUint32(uint32(l), b)
	copy(b[4:], pkg)

	pl := PacketSize - 4
	ln := int(math.Ceil(float64(l) / float64(pl)))
	pkts := make([][]byte, ln)
	n := 0
	ids := serial.MarshalUint32(id, nil)
	for ; n < ln-1; n++ {
		pkts[n] = make([]byte, PacketSize)
		copy(pkts[n], ids)
		copy(pkts[n][4:], b[n*pl:(n+1)*pl])
	}
	final := b[n*pl:]
	pkts[n] = make([]byte, len(final)+4)
	copy(pkts[n], ids)
	copy(pkts[n][4:], final)

	return pkts
}
