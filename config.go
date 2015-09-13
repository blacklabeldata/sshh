package sshh

import (
	"sync"
	"time"

	log "github.com/mgutz/logxi/v1"
	"golang.org/x/crypto/ssh"
)

// Config is used to setup the SSHServer, including the server config and the SSHHandlers.
type Config struct {
	sync.Mutex

	// Deadline is the maximum time the listener will block
	// between connections. As a consequence, this duration
	// also sets the max length of time the SSH server will
	// be unresponsive before shutting down.
	Deadline time.Duration

	// Handlers is a map of SSHHandlers which process incoming connections. The map
	// consists of channel names as keys and SSHHandlers as the values. If a
	// client connects and creates a channel with a defined SSHHandler, the handler
	// will process all requests on that channel. If a channel is accepted without a
	// defined handler, the channel is closed as well as the connection.
	Handlers map[string]SSHHandler

	// Logger logs errors and debug output for the SSH server.
	Logger log.Logger

	// Bind specifies the Bind address the SSH server will listen on.
	Bind string

	// PrivateKey is added to the SSH config as a host key.
	PrivateKey ssh.Signer

	// AuthLogCallback, if non-nil, is called to log all authentication
	// attempts.
	AuthLogCallback func(conn ssh.ConnMetadata, method string, err error)

	// PasswordCallback, if non-nil, is called when a user
	// attempts to authenticate using a password.
	PasswordCallback func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error)

	// PublicKeyCallback, if non-nil, is called when a client attempts public
	// key authentication. It must return true if the given public key is
	// valid for the given user. For example, see CertChecker.Authenticate.
	PublicKeyCallback func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error)

	// sshConfig is used to verify incoming connections.
	sshConfig *ssh.ServerConfig
}

// SSHConfig returns an SSH server configuration. If the AuthLogCallback is nil at the
// time this method is called, the default function will be used.
func (c *Config) SSHConfig() *ssh.ServerConfig {

	// Create server config
	sshConfig := &ssh.ServerConfig{
		NoClientAuth:      false,
		PasswordCallback:  c.PasswordCallback,
		PublicKeyCallback: c.PublicKeyCallback,
		AuthLogCallback:   c.AuthLogCallback,
	}
	sshConfig.AddHostKey(c.PrivateKey)
	return sshConfig
}

// Handler is a helper method for determining if a channel handler has been defined.
func (c *Config) Handler(channel string) (handler SSHHandler, ok bool) {
	c.Lock()
	handler, ok = c.Handlers[channel]
	c.Unlock()
	return
}
