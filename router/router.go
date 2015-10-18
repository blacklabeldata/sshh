package router

import (
	"errors"

	log "github.com/mgutz/logxi/v1"
)

var ErrUnknownChannel = errors.New("Unknown channel type")

func New(l log.Logger, panicHandler PanicHandler, notFound Handler) *Router {
	return &Router{
		root: new(node),
		// logger:       l,
		PanicHandler: panicHandler,
		NotFound:     notFound,
	}
}

type Router struct {
	root         *node
	logger       log.Logger
	PanicHandler PanicHandler
	NotFound     Handler
}

func (r *Router) Register(path string, handle Handler) {
	r.root.addRoute(path, handle)
}

func (r *Router) RegisterFunc(path string, handle HandlerFunc) {
	r.Register(path, &basicHandler{handle})
}

func (r *Router) HasRoute(path string) bool {
	if handler, _, _ := r.root.getValue(path); handler != nil {
		return true
	}
	return false
}

func (r *Router) GetRoute(path string) (Handler, Params, bool) {
	if handler, params, _ := r.root.getValue(path); handler != nil {
		return handler, params, true
	}
	return nil, nil, false
}

func (r *Router) callRoute(c *Context) (err error, called bool) {
	if handler, params, ok := r.GetRoute(c.Path); ok {
		c.Params = params
		err = handler.Handle(c)
		called = true
	}
	return
}

func (r *Router) Handle(c *Context) error {
	if r.PanicHandler != nil {
		defer r.recv(c)
	}

	err, ok := r.callRoute(c)
	if ok {
		return err
	} else if c.Path != "/" {

		// Try to fix the request path
		fixedPath, found := r.root.findCaseInsensitivePath(CleanPath(c.Path), true)
		if found {
			c.Path = string(fixedPath)
			err, ok = r.callRoute(c)
			if ok {
				return err
			}
		}
	}

	// Handle unknown path
	if r.NotFound != nil {
		r.NotFound.Handle(c)
	} else {
		return ErrUnknownChannel
	}
	return nil
}

func (r *Router) recv(c *Context) {
	if rcv := recover(); rcv != nil {
		r.PanicHandler.Handle(c, rcv)
	}
}
