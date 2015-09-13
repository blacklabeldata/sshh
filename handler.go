package sshh

import (
	"golang.org/x/crypto/ssh"
	tomb "gopkg.in/tomb.v2"
)

// A SSHHandler is registered with an SSH server to process incoming connections on a particular channel.
type SSHHandler interface {

	// Handle processes SSH connections. The requests chan can be disregarded if it is not needed.
	Handle(t tomb.Tomb, conn *ssh.ServerConn, channel ssh.Channel, requests <-chan *ssh.Request) error
}
