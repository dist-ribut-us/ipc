package ipc

import (
	"github.com/dist-ribut-us/errors"
	"github.com/dist-ribut-us/log"
	"github.com/dist-ribut-us/rnet"
	"github.com/golang/protobuf/proto"
)

const (
	// MaskQuery is applied to a type if it is a query type
	MaskQuery = uint32(1 << (iota))
	// MaskResponse is applied to a type if it is a response type
	MaskResponse
)

// message index
const (
	TUndefined = uint32(iota)
	TPort
	TPing
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
	var h Header
	err := proto.Unmarshal(m.Body, &h)
	if err != nil {
		return nil, err
	}
	base := &Base{
		Header: &h,
		port:   m.Addr.Port(),
		ID:     m.ID,
		Buf:    m.Body,
		proc:   m.proc,
	}
	return base, nil
}

// Base provides a base message type. It wraps ipc.Header and provides helper
// functions for simple query and response messages.
type Base struct {
	*Header
	port rnet.Port
	ID   uint32
	Buf  []byte
	proc *Proc
}

// To sets the port Send will send to.
func (b *Base) To(port rnet.Port) *Base {
	b.port = port
	return b
}

// Port gets the port on Base. This is the port that base will send to with send
// or respond. If base was derived from a message, this is the port the message
// was received from.
func (b *Base) Port() rnet.Port {
	return b.port
}

// GetID returns the ID and fulfills Query.
func (b *Base) GetID() uint32 {
	return b.ID
}

// IsQuery checks if the underlying type is a query
func (h *Header) IsQuery() bool {
	return h.Flags&MaskQuery == MaskQuery
}

// IsResponse checks if the underlying type is a query
func (h *Header) IsResponse() bool {
	return h.Flags&MaskResponse == MaskResponse
}

// Respond to a query
func (b *Base) Respond(body []byte) {
	r := &Header{
		Type:  b.Type,
		Flags: MaskResponse,
		Body:  body,
	}
	b.proc.SendResponse(r, b)
}

// SetFlags exponses Flags for the FlagsGetterSetter interface
func (h *Header) SetFlags(f uint32) {
	h.Flags = f
}

// Unmarshal wraps proto.Unmarshal and uses the underlying buffer
func (b *Base) Unmarshal(pb proto.Message) error {
	return proto.Unmarshal(b.Buf, pb)
}

// Query creates a basic query.
func (p *Proc) Query(t uint32, body []byte) *Base {
	return &Base{
		Header: &Header{
			Type:  t,
			Flags: MaskQuery,
			Body:  body,
		},
		proc: p,
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
		b.proc.Send(buf, b.port)
	} else {
		b.proc.SendQuery(b.Header, b.port, callback)
	}
}
