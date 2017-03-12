package ipc

import (
	"crypto/rand"
	"github.com/dist-ribut-us/rnet"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
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

func TestResponseCallback(t *testing.T) {
	p1, err := RunNew(1236)
	assert.NoError(t, err)
	p2, err := RunNew(1237)
	assert.NoError(t, err)

	out := make(chan string)

	go func() {
		m := <-p2.Chan()
		q, err := m.Unwrap()
		if !assert.NoError(t, err) {
			out <- "fail"
			return
		}
		if !assert.Equal(t, Type_QUERY, q.Type) {
			out <- "fail"
			return
		}
		assert.Equal(t, "test", q.Query.Type)
		assert.Equal(t, []byte{111}, q.Query.Body)
		r := &Response{
			Body: []byte("testbody"),
		}
		p2.SendResponse(r, q)
	}()

	q := &Query{
		Type: "test",
		Body: []byte{111},
	}
	p1.SendQuery(q, p2.Port(), func(r *Wrapper) {
		out <- string(r.Response.Body)
	})

	select {
	case resp := <-out:
		assert.Equal(t, "testbody", resp)
	case <-time.After(time.Millisecond * 50):
		t.Error("time out")
	}

}
