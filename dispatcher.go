package sshh

import (
	"fmt"
	"net/url"

	"github.com/blacklabeldata/sshh/router"
	log "github.com/mgutz/logxi/v1"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
)

type Dispatcher interface {
	Dispatch(context.Context, *ssh.ServerConn, ssh.NewChannel)
}

type SimpleDispatcher struct {
	Logger       log.Logger
	Handlers     map[string]Handler
	PanicHandler PanicHandler
	NotFound     Handler
}

func (u *SimpleDispatcher) Dispatch(c context.Context, conn *ssh.ServerConn, ch ssh.NewChannel) {
	defer conn.Close()

	var ctx *Context
	if u.PanicHandler != nil {
		if rcv := recover(); rcv != nil {
			u.PanicHandler.Handle(ctx, rcv)
		}
	}

	// Get channel type
	chType := ch.ChannelType()

	handler, ok := u.Handlers[chType]
	if !ok {
		return
	}

	// Otherwise, accept the channel
	channel, requests, err := ch.Accept()
	if err != nil {
		u.Logger.Warn("Error creating channel", "type", chType, "err", err)
		ch.Reject(ChannelAcceptError, chType)
		return
	}

	// Handle the channel
	ctx = &Context{
		Context:  c,
		Channel:  channel,
		Requests: requests,
	}
	err = handler.Handle(ctx)
	if err != nil {
		u.Logger.Warn("Error handling channel", "type", chType, "err", err)
		ch.Reject(ChannelHandleError, fmt.Sprintf("error handling channel: %s", err.Error()))
		return
	}
}

type UrlDispatcher struct {
	Logger log.Logger
	Router *router.Router
}

func (u *UrlDispatcher) Dispatch(c context.Context, conn *ssh.ServerConn, ch ssh.NewChannel) {
	defer conn.Close()

	// Get channel type
	chType := ch.ChannelType()

	// Parse channel URI
	uri, err := url.ParseRequestURI(chType)
	if err != nil {
		u.Logger.Warn("Error parsing channel type", "type", chType, "err", err)
		ch.Reject(InvalidChannelType, "invalid channel URI")
		return
	} else if reject(chType, uri, ch, u.Logger) {
		return
	}
	chType = uri.Path

	// Parse query params
	values, err := url.ParseQuery(uri.RawQuery)
	if err != nil {
		u.Logger.Warn("Error parsing query params", "values", values, "err", err)
		ch.Reject(InvalidQueryParams, "invalid query params in channel type")
		return
	}

	// Determine if channel is acceptable (has a registered handler)
	if !u.Router.HasRoute(chType) {
		u.Logger.Info("UnknownChannelType", "type", chType)
		ch.Reject(ssh.UnknownChannelType, chType)
		return
	}

	// Otherwise, accept the channel
	channel, requests, err := ch.Accept()
	if err != nil {
		u.Logger.Warn("Error creating channel", "type", chType, "err", err)
		ch.Reject(ChannelAcceptError, chType)
		return
	}

	// Handle the channel
	err = u.Router.Handle(&router.UrlContext{
		Path:     uri.Path,
		Context:  c,
		Values:   values,
		Channel:  channel,
		Requests: requests,
	})
	if err != nil {
		u.Logger.Warn("Error handling channel", "type", chType, "err", err)
		ch.Reject(ChannelHandleError, fmt.Sprintf("error handling channel: %s", err.Error()))
		return
	}
}

func reject(chType string, uri *url.URL, ch ssh.NewChannel, logger log.Logger) bool {
	if uri.Scheme != "" {
		logger.Warn("URI schemes not supported", "type", chType)
		ch.Reject(SchemeNotSupported, "schemes are not supported in the channel URI")
		return true
	} else if uri.User != nil {
		logger.Warn("URI users not supported", "type", chType)
		ch.Reject(UserNotSupported, "users are not supported in the channel URI")
		return true
	} else if uri.Host != "" {
		logger.Warn("URI hosts not supported", "type", chType)
		ch.Reject(HostNotSupported, "hosts are not supported in the channel URI")
		return true
	}
	return false
}
