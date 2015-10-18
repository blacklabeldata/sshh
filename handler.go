package sshh

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
	ChannelType string
	Context     context.Context
	Channel     ssh.Channel
	Requests    <-chan *ssh.Request
}

type RequestConsumer interface {
	Consume(<-chan *ssh.Request)
}
