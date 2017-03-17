package ipc

import (
	"github.com/dist-ribut-us/errors"
	"github.com/dist-ribut-us/log"
	"github.com/dist-ribut-us/message"
	"github.com/dist-ribut-us/rnet"
	"github.com/dist-ribut-us/serial"
	"github.com/golang/protobuf/proto"
)

// ErrTypesDoNotMatch is thrown when trying to convert a Type to the wrong
// type
const ErrTypesDoNotMatch = errors.String("Types do not match")

// Message is used to assemble the messages and send them through the channel
// when they are complete.
type Message struct {
	ID   uint32
	Body []byte
	Addr *rnet.Addr
	Len  int
	proc *Proc
}

// ToBase a message body to get it's type
func (m *Message) ToBase() (*Base, error) {
	var h message.Header
	err := proto.Unmarshal(m.Body, &h)
	if err != nil {
		return nil, err
	}
	base := &Base{
		Header: &h,
		ID:     m.ID,
		proc:   m.proc,
		port:   m.Addr.Port(),
	}
	return base, nil
}

// Base provides a base message type. It wraps message.Header and provides
// helper functions for simple query and response messages.
type Base struct {
	*message.Header
	ID   uint32
	proc *Proc
	port rnet.Port
}

// To sets the port Send will send to.
func (b *Base) To(port rnet.Port) *Base {
	b.port = port
	return b
}

// Port returns the base port - this is the ipc port that the message came from
// or that it is sent to send to.
func (b *Base) Port() rnet.Port {
	return b.port
}

// GetID returns the ID and fulfills Query.
func (b *Base) GetID() uint32 {
	return b.ID
}

// Respond to a query
func (b *Base) Respond(body []byte) {
	r := &message.Header{
		Type32: b.Type32,
		Flags:  uint32(message.ResponseFlag),
		Body:   body,
	}
	b.proc.SendResponse(r, b)
}

// Unmarshal the body of the header
func (b *Base) Unmarshal(pb proto.Message) error {
	return proto.Unmarshal(b.Body, pb)
}

// Query creates a basic query.
func (p *Proc) Query(t message.Type, body interface{}) *Base {
	h := &message.Header{
		Type32: uint32(t),
		Flags:  uint32(message.QueryFlag),
	}
	h.SetBody(body)
	return &Base{
		Header: h,
		proc:   p,
	}
}

// Base creates a basic message with no flags. Body can be either a proto
// message or a byte slice.
func (p *Proc) Base(t message.Type, body interface{}) *Base {
	h := &message.Header{
		Type32: uint32(t),
	}
	h.SetBody(body)
	return &Base{
		Header: h,
		ID:     randID(),
		proc:   p,
	}
}

// Query defines the fields needed for SendResponse to respond to query
type Query interface {
	GetID() uint32
	Port() rnet.Port
}

// SendResponse takes a response and the the wrapped query it is responding to
// and sends the response with the same message id to source of the query.
func (p *Proc) SendResponse(r proto.Message, q Query) {
	msg, err := proto.Marshal(r)
	if log.Error(errors.Wrap("wrapping_response_to_send", err)) {
		return
	}
	pkts := p.pktr.MakeWithID(q.GetID(), msg)
	addr := q.Port().On("127.0.0.1")
	if log.Error(errors.Wrap("generating_local_addr_for_ipc", addr.Err)) {
		return
	}
	errs := p.srv.SendAll(pkts, addr)
	if errs != nil {
		log.Info(log.Lbl("while_sending_response_over_ipc"), errs)
	}
}

// Callback is used when sending a query
type Callback func(r *Base)

// SendQuery will send a query to another process and send the response to the
// callback function. This also means that the response will not end up on the
// Proc channel.
func (p *Proc) SendQuery(q proto.Message, port rnet.Port, callback Callback) {
	msg, err := proto.Marshal(q)
	if log.Error(errors.Wrap("wrapping_query_to_send", err)) {
		return
	}
	id := randID()
	pkts := p.pktr.MakeWithID(id, msg)
	addr := port.On("127.0.0.1")
	if log.Error(errors.Wrap("generating_local_addr_for_ipc", addr.Err)) {
		return
	}
	p.pktr.SetCallback(id, callback)
	errs := p.srv.SendAll(pkts, addr)
	if errs != nil {
		log.Info(log.Lbl("while_sending_response_over_ipc"), errs)
	}
}

// Send a message. If callback is not nil, the reponse will be sent to the
// callback
func (b *Base) Send(callback Callback) {
	if callback == nil {
		buf, err := proto.Marshal(b.Header)
		if log.Error(err) {
			return
		}
		b.proc.Send(buf, b.Port())
	} else {
		b.proc.SendQuery(b.Header, b.Port(), callback)
	}
}

// RequestServicePort is a shorthand to request a service port from pool.
func (p *Proc) RequestServicePort(serviceName string, pool rnet.Port, callback Callback) {
	p.
		Query(message.GetPort, []byte(serviceName)).
		To(pool).
		Send(callback)
}

// RegisterWithOverlay is a shorthand to register a service with overlay.
func (p *Proc) RegisterWithOverlay(id uint32, overlay rnet.Port, callback Callback) {
	p.
		Base(message.RegisterService, serial.MarshalUint32(id, nil)).
		To(overlay).
		Send(callback)
}
