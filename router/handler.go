package router

import (
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
)

type Handler interface {
	Handle(*Context) error
}

type HandlerFunc func(*Context) error

type basicHandler struct {
	hf HandlerFunc
}

func (b *basicHandler) Handle(c *Context) error {
	return b.hf(c)
}

type PanicHandler interface {
	Handle(*Context, interface{})
}

type Context struct {
	Path     string
	Params   Params
	Context  context.Context
	Channel  ssh.Channel
	Requests <-chan *ssh.Request
}

type Param struct {
	Key, Value string
}

type Params []Param

func (p Params) ByName(key string) string {
	for _, param := range p {
		if param.Key == key {
			return param.Value
		}
	}
	return ""
}
