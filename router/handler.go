package router

import (
	"net/url"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
)

type Handler interface {
	Handle(*UrlContext) error
}

type HandlerFunc func(*UrlContext) error

type basicHandler struct {
	hf HandlerFunc
}

func (b *basicHandler) Handle(c *UrlContext) error {
	return b.hf(c)
}

type PanicHandler interface {
	Handle(*UrlContext, interface{})
}

type UrlContext struct {
	Path     string
	Params   Params
	Values   url.Values
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
