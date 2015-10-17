package sshh

import "golang.org/x/crypto/ssh"

const (
	ChannelAcceptError ssh.RejectionReason = iota + 1000
	InvalidChannelType
	InvalidQueryParams
	HostNotSupported
	SchemeNotSupported
	UserNotSupported
	ChannelHandleError
)
