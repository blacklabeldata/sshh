package sshh

import "golang.org/x/crypto/ssh"

const (
	ChannelAcceptError ssh.RejectionReason = 1000
	InvalidChannelType ssh.RejectionReason = 1001
	InvalidQueryParams ssh.RejectionReason = 1002
	HostNotSupported   ssh.RejectionReason = 1003
	SchemeNotSupported ssh.RejectionReason = 1004
	UserNotSupported   ssh.RejectionReason = 1005
	ChannelHandleError ssh.RejectionReason = 1006
)
