package ipc

import (
	"github.com/dist-ribut-us/log"
	"github.com/dist-ribut-us/rnet"
	"github.com/dist-ribut-us/serial"
	"math"
	"time"
)

// PacketSize is the max packet size, it's set a bit less than the absolute max
// at a nice, round value.
var PacketSize = 50000

// packeter handles making and collecting packets for inter-process
// communicaiton
type packeter struct {
	packets       *packets
	callbacks     *callbacks
	proc          *Proc
	packetHandler func(*Package)
	baseHandler   func(*Base)
}

func newPacketer(proc *Proc) *packeter {
	return &packeter{
		packets:   newpackets(),
		callbacks: newcallbacks(),
		proc:      proc,
	}
}

func (p *packeter) setCallback(id uint32, callback Callback) {
	p.callbacks.set(id, callback)
	go p.cleanupCallback(id)
}

func (p *packeter) cleanupCallback(id uint32) {
	time.Sleep(time.Millisecond * 20)
	_, timedout := p.callbacks.get(id)
	if timedout {
		p.callbacks.delete(id)
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
	pkg, ok := p.packets.get(id)
	if !ok {
		pkg = &Package{
			ID:   id,
			Len:  int(serial.UnmarshalUint32(b[4:])),
			Addr: addr,
			Body: b[8:],
			proc: p.proc,
		}
		// don't call p.packets.set here, it gets called at the bottom if the
		// message is more than one packet long, which it often isn't
	} else if addr.Port() == pkg.Addr.Port() {
		pkg.Body = append(pkg.Body, b[4:]...)
	} else {
		log.Info(log.Lbl("message_changed_ports"), log.KV{"started_on", pkg.Addr}, log.KV{"now_on", addr})
		return
	}

	if len(pkg.Body) >= pkg.Len {
		callback, ok := p.callbacks.get(pkg.ID)
		if ok {
			b, err := pkg.ToBase()
			if !log.Error(err) && b.IsResponse() {
				p.callbacks.delete(id)
				go callback(b)
				p.packets.delete(id)
				return
			}
		}

		if p.packetHandler != nil {
			p.packetHandler(pkg)
		} else if p.baseHandler != nil {
			b, err := pkg.ToBase()
			if !log.Error(err) {
				go p.baseHandler(b)
			}
		} else {
			log.Info(log.Lbl("unhandled_message"))
		}

		p.packets.delete(id)
	} else {
		p.packets.set(id, pkg)
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
