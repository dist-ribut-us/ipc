package ipc

import (
	"crypto/rand"
	"github.com/dist-ribut-us/rnet"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRoundTripOnePacket(t *testing.T) {
	ln := 1000
	p := &Packeter{
		packets: make(map[uint32]*Message),
		ch:      make(chan *Message),
	}
	msg := make([]byte, ln)
	rand.Read(msg)
	pks := p.Make(msg)
	addr, err := rnet.ResolveAddr("127.0.0.1:1234")
	assert.NoError(t, err)
	go func() {
		for _, pkt := range pks {
			p.Receive(pkt, addr)
		}
	}()

	out := <-p.Chan()
	assert.Equal(t, msg, out.Body)
	assert.Equal(t, addr.String(), out.Addr.String())
}

func TestRoundTripManyPackets(t *testing.T) {
	ln := PacketSize*3 + 2000
	p := &Packeter{
		packets: make(map[uint32]*Message),
		ch:      make(chan *Message),
	}
	msg := make([]byte, ln)
	rand.Read(msg)
	pks := p.Make(msg)
	addr, err := rnet.ResolveAddr("127.0.0.1:1234")
	assert.NoError(t, err)
	go func() {
		for _, pkt := range pks {
			p.Receive(pkt, addr)
		}
	}()

	out := <-p.Chan()
	assert.Equal(t, msg, out.Body)
	assert.Equal(t, addr.String(), out.Addr.String())
}

func TestIPCRoundTrip(t *testing.T) {
	p1, err := New(1234)
	assert.NoError(t, err)
	p2, err := RunNew(1235)
	assert.NoError(t, err)

	ln := 1000
	msg := make([]byte, ln)
	rand.Read(msg)
	p1.Send(msg, 1235)

	msgOut := <-p2.Chan()
	assert.Equal(t, msg, msgOut.Body)
}
