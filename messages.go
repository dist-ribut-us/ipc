package ipc

import (
	"github.com/dist-ribut-us/errors"
	"github.com/dist-ribut-us/rnet"
	"github.com/golang/protobuf/proto"
)

// ErrTypesDoNotMatch is thrown when trying to convert a Wrapper to the wrong
// type
const ErrTypesDoNotMatch = errors.String("Types do not match")

// Message is used to assemble the messages and send them through the channel
// when they are complete.
type Message struct {
	ID   uint32
	Body []byte
	Addr *rnet.Addr
	Len  int
}

// Unwrap a message body to get it's type
func (m *Message) Unwrap() (*Wrapper, error) {
	var w Wrapper
	err := proto.Unmarshal(m.Body, &w)
	if err != nil {
		return nil, err
	}
	w.Port32 = uint32(m.Addr.Port())
	w.Id = m.ID
	return &w, nil
}

// Port returns the port that the query came from
func (w *Wrapper) Port() rnet.Port {
	return rnet.Port(w.Port32)
}

// Wrap a query into a wrapped byte slice
func (q *Query) Wrap() ([]byte, error) {
	return proto.Marshal(&Wrapper{
		Type:  Type_QUERY,
		Query: q,
	})
}

// Wrap a query into a wrapped byte slice
func (r *Response) Wrap() ([]byte, error) {
	return proto.Marshal(&Wrapper{
		Type:     Type_RESPONSE,
		Response: r,
	})
}
