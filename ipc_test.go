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
	p := newPacketer(nil)
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
	p := newPacketer(nil)
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
	testType := uint32(3141)

	p1, err := RunNew(1236)
	assert.NoError(t, err)
	p2, err := RunNew(1237)
	assert.NoError(t, err)

	out := make(chan string)

	go func() {
		q, err := (<-p2.Chan()).ToBase()
		if !assert.NoError(t, err) || !assert.True(t, q.IsQuery()) {
			out <- "fail"
			return
		}
		assert.Equal(t, testType, q.Type)
		assert.Equal(t, []byte{111}, q.Body)
		q.Respond([]byte("testbody"))
	}()

	p1.
		Query(testType, []byte{111}).
		To(p2.Port()).
		Send(func(r *Base) {
			assert.Equal(t, testType, r.Type)
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
