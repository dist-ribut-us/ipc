package ipc

import (
	"crypto/rand"
	"github.com/dist-ribut-us/log"
	"github.com/dist-ribut-us/rnet"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func init() {
	log.Mute()
}

func TestRoundTripOnePacket(t *testing.T) {
	ln := 1000
	p := newPacketer(nil)
	msg := make([]byte, ln)
	rand.Read(msg)
	pks := p.make(987654321, msg)
	addr, err := rnet.ResolveAddr("127.0.0.1:1234")
	assert.NoError(t, err)
	go func() {
		for _, pkt := range pks {
			p.Receive(pkt, addr)
		}
	}()
	ch := make(chan *Package)
	p.handler = func(msg *Package) {
		ch <- msg
	}

	select {
	case out := <-ch:
		assert.Equal(t, msg, out.Body)
		assert.Equal(t, addr.String(), out.Addr.String())
	case <-time.After(time.Millisecond * 10):
		t.Error("timeout")
	}
}

func TestRoundTripManyPackets(t *testing.T) {
	ln := PacketSize*3 + 2000
	p := newPacketer(nil)
	msg := make([]byte, ln)
	rand.Read(msg)
	pks := p.make(987654321, msg)
	addr, err := rnet.ResolveAddr("127.0.0.1:1234")
	assert.NoError(t, err)
	go func() {
		for _, pkt := range pks {
			p.Receive(pkt, addr)
		}
	}()

	ch := make(chan *Package)
	p.handler = func(msg *Package) {
		ch <- msg
	}

	select {
	case out := <-ch:
		assert.Equal(t, msg, out.Body)
		assert.Equal(t, addr.String(), out.Addr.String())
	case <-time.After(time.Millisecond * 10):
		t.Error("timeout")
	}
}

func TestIPCRoundTrip(t *testing.T) {
	p1, err := New(1234)
	assert.NoError(t, err)
	p2, err := RunNew(1235)
	assert.NoError(t, err)

	ch := make(chan *Package)
	p2.pktr.handler = func(msg *Package) {
		ch <- msg
	}

	ln := 1000
	msg := make([]byte, ln)
	rand.Read(msg)
	p1.Send(6789, msg, 1235)

	select {
	case out := <-ch:
		assert.Equal(t, msg, out.Body)
	case <-time.After(time.Millisecond * 20):
		t.Error("timed out")
	}
}

func TestSelfSend(t *testing.T) {
	// Overlay sends to itself sometimes
	ln := 1000
	p1, err := RunNew(3333)
	assert.NoError(t, err)

	msg := make([]byte, ln)
	rand.Read(msg)

	ch := make(chan *Package)
	p1.pktr.handler = func(msg *Package) {
		ch <- msg
	}

	p1.Send(12345, msg, p1.GetPort())

	select {
	case out := <-ch:
		assert.Equal(t, msg, out.Body)
	case <-time.After(time.Millisecond * 20):
		t.Error("timeout")
	}
}
