package ipc

import (
	"crypto/rand"
	"github.com/dist-ribut-us/log"
	"github.com/dist-ribut-us/message"
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
	p.packetHandler = func(msg *Package) {
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
	p.packetHandler = func(msg *Package) {
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

func TestHandler(t *testing.T) {
	ln := PacketSize*3 + 2000
	p := newPacketer(nil)

	ch := make(chan *message.Header)
	p.baseHandler = func(b *Base) {
		ch <- b.Header
	}

	msg := message.NewHeader(message.Test, make([]byte, ln))
	rand.Read(msg.Body)
	var id uint32 = 12354678
	pks := p.make(id, msg.Marshal())

	addr := rnet.Port(1234).Addr()
	go func() {
		for _, pkt := range pks {
			p.Receive(pkt, addr)
		}
	}()

	select {
	case out := <-ch:
		assert.Equal(t, msg.Body, out.Body)
		assert.Equal(t, msg.Type32, out.Type32)
		assert.Equal(t, id, out.Id)
	case <-time.After(time.Millisecond * 20):
		t.Error("timed out")
	}
}

func TestIPCRoundTrip(t *testing.T) {
	p1, err := New(1234)
	assert.NoError(t, err)
	p2, err := RunNew(1235)
	assert.NoError(t, err)

	ch := make(chan *Package)
	p2.pktr.packetHandler = func(msg *Package) {
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

func TestResponseCallback(t *testing.T) {
	p1, err := RunNew(1236)
	assert.NoError(t, err)
	p2, err := RunNew(1237)
	assert.NoError(t, err)

	out := make(chan string)

	p2.pktr.baseHandler = func(q *Base) {
		if !assert.True(t, q.IsQuery()) {
			out <- "fail"
			return
		}
		assert.Equal(t, message.Test, q.GetType())
		assert.Equal(t, []byte{111}, q.Body)
		q.Respond([]byte("testbody"))
	}

	p1.
		Query(message.Test, []byte{111}).
		To(p2.Port()).
		Send(func(r *Base) {
			assert.Equal(t, message.Test, r.GetType())
			assert.True(t, r.IsResponse())
			out <- string(r.Body)
		})

	select {
	case resp := <-out:
		assert.Equal(t, "testbody", resp)
	case <-time.After(time.Millisecond * 50):
		t.Error("time out")
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
	p1.pktr.packetHandler = func(msg *Package) {
		ch <- msg
	}

	p1.Send(12345, msg, p1.Port())

	select {
	case out := <-ch:
		assert.Equal(t, msg, out.Body)
	case <-time.After(time.Millisecond * 20):
		t.Error("timeout")
	}
}
