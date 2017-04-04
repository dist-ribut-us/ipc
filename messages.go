package ipc

import (
	"github.com/dist-ribut-us/errors"
	"github.com/dist-ribut-us/log"
	"github.com/dist-ribut-us/message"
	"github.com/dist-ribut-us/rnet"
)

// ErrTypesDoNotMatch is thrown when trying to convert a Type to the wrong
// type
const ErrTypesDoNotMatch = errors.String("Types do not match")

// Package is used to assemble the messages and send them through the channel
// when they are complete.
type Package struct {
	ID   uint32
	Body []byte
	Addr *rnet.Addr
	Len  int
	proc *Proc
}

// ToBase a message body to get it's type
func (m *Package) ToBase() (*Base, error) {
	h := message.Unmarshal(m.Body)
	if h == nil {
		return nil, nil
	}
	h.Id = m.ID
	base := &Base{
		Header: h,
		proc:   m.proc,
		port:   m.Addr.Port(),
	}
	return base, nil
}

// Base provides a base message type. It wraps message.Header and provides
// helper functions for simple query and response messages.
type Base struct {
	*message.Header
	proc *Proc
	port rnet.Port
}

// To sets the port Send will send to.
func (b *Base) To(port rnet.Port) *Base {
	b.port = port
	return b
}

// ToNet sets the fields for a message to be sent to the Overlay service and
// then out over the net.
func (b *Base) ToNet(overlayPort rnet.Port, netAddr *rnet.Addr, serviceID uint32) *Base {
	return b.
		SetFlag(message.ToNet).
		SetAddr(netAddr).
		SetService(serviceID).
		To(overlayPort)
}

// SetAddr sets the address on a message. This indicates the network address
// where the message should be sent, most likely by overlay.
func (b *Base) SetAddr(addr *rnet.Addr) *Base {
	b.Header.SetAddr(addr)
	return b
}

// SetFlag field on the Header
func (b *Base) SetFlag(flag message.BitFlag) *Base {
	b.Header.SetFlag(flag)
	return b
}

// SetService field on the header
func (b *Base) SetService(service uint32) *Base {
	b.Header.Service = service
	return b
}

// Port returns the base port - this is the ipc port that the message came from
// or that it is sent to send to.
func (b *Base) Port() rnet.Port {
	return b.port
}

// GetID returns the ID and fulfills Query.
func (b *Base) GetID() uint32 {
	return b.Id
}

// Respond to a query
func (b *Base) Respond(body interface{}) {
	r := &message.Header{
		Type32: b.Type32,
		Flags:  uint32(message.ResponseFlag),
	}
	r.SetBody(body)

	if b.IsFromNet() {
		r.SetFlag(message.ToNet)
		r.Addrpb = b.Addrpb
	}

	pkts := b.proc.pktr.make(b.Id, r.Marshal())
	addr := b.Port().Local()
	if log.Error(errors.Wrap("generating_local_addr_for_ipc", addr.Err)) {
		return
	}
	errs := b.proc.srv.SendAll(pkts, addr)
	if errs != nil {
		log.Info(log.Lbl("while_sending_response_over_ipc"), errs, log.Line(-1))
	}
}

// Query creates a basic query.
func (p *Proc) Query(t message.Type, body interface{}) *Base {
	h := message.NewHeader(t, body)
	h.SetFlag(message.QueryFlag)
	return &Base{
		Header: h,
		proc:   p,
	}
}

// Base creates a basic message with no flags. Body can be either a proto
// message or a byte slice.
func (p *Proc) Base(t message.Type, body interface{}) *Base {
	return &Base{
		Header: message.NewHeader(t, body),
		proc:   p,
	}
}

// Query defines the fields needed for SendResponse to respond to query
type Query interface {
	GetID() uint32
	Port() rnet.Port
}

// Callback is used when sending a query
type Callback func(r *Base)

// Send a message. If callback is not nil, the reponse will be sent to the
// callback
func (b *Base) Send(callback Callback) {
	id := b.Id
	b.Id = 0

	if callback != nil {
		b.proc.pktr.setCallback(id, callback)
	}

	b.proc.Send(id, b.Marshal(), b.Port())
}

// RequestServicePort is a shorthand to request a service port from pool.
func (p *Proc) RequestServicePort(serviceName string, pool rnet.Port, callback Callback) {
	p.
		Query(message.GetPort, serviceName).
		To(pool).
		Send(callback)
}

// RegisterWithOverlay is a shorthand to register a service with overlay.
func (p *Proc) RegisterWithOverlay(serviceID uint32, overlay rnet.Port) {
	p.
		Base(message.RegisterService, serviceID).
		To(overlay).
		Send(nil)
}
