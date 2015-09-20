package sshh

import (
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
	tomb "gopkg.in/tomb.v2"
)

// NewSSHServer creates a new server with the given config. The server will call `cfg.SSHConfig()` to setup
// the server. If an error occurs it will be returned. If the Bind address is empty or invalid
// an error will be returned. If there is an error starting the TCP server, the error will be returned.
func NewSSHServer(cfg *Config) (server SSHServer, err error) {

	// Create ssh config for server
	sshConfig := cfg.SSHConfig()
	cfg.sshConfig = sshConfig

	// Validate the ssh bind addr
	if cfg.Bind == "" {
		err = fmt.Errorf("ssh server: Empty SSH bind address")
		return
	}

	// Open SSH socket
	sshAddr, e := net.ResolveTCPAddr("tcp", cfg.Bind)
	if e != nil {
		err = fmt.Errorf("ssh server: Invalid tcp address")
		return
	}

	// Create listener
	listener, e := net.ListenTCP("tcp", sshAddr)
	if e != nil {
		err = e
		return
	}
	server.listener = listener
	server.Addr = listener.Addr().(*net.TCPAddr)
	server.config = cfg
	return
}

// SSHServer handles all the incoming connections as well as handler dispatch.
type SSHServer struct {
	config   *Config
	Addr     *net.TCPAddr
	listener *net.TCPListener
	t        tomb.Tomb
}

// Start starts accepting client connections. This method is non-blocking.
func (s *SSHServer) Start() {
	s.config.Logger.Info("Starting SSH server", "addr", s.config.Bind)
	s.t.Go(s.listen)
}

// Stop stops the server and kills all goroutines. This method is blocking.
func (s *SSHServer) Stop() error {
	s.t.Kill(nil)
	s.config.Logger.Info("Shutting down SSH server...")
	return s.t.Wait()
}

// listen accepts new connections and handles the conversion from TCP to SSH connections.
func (s *SSHServer) listen() error {
	defer s.listener.Close()

	// Create tomb for connection goroutines
	var t tomb.Tomb
	t.Go(func() error {
	OUTER:
		for {

			// Accepts will only block for 1s
			s.listener.SetDeadline(time.Now().Add(s.config.Deadline))

			select {

			// Stop server on channel receive
			case <-s.t.Dying():
				t.Kill(nil)
				break OUTER
			default:

				// Accept new connection
				tcpConn, err := s.listener.Accept()
				if err != nil {
					if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
						s.config.Logger.Trace("Connection timeout...")
					} else {
						s.config.Logger.Warn("Connection failed", "error", err)
					}
					continue
				}

				// Handle connection
				s.config.Logger.Info("Successful TCP connection:", tcpConn.RemoteAddr().String())
				t.Go(func() error {

					// Return the error for the
					err := s.handleTCPConnection(t, tcpConn)
					if err != io.EOF {
						return err
					}
					return nil
				})
			}
		}
		return nil
	})

	return t.Wait()
}

// handleTCPConnection converts the TCP connection into an ssh.ServerConn and processes all incoming channel requests
// and dispatches them to defined handlers if they exist.
func (s *SSHServer) handleTCPConnection(parentTomb tomb.Tomb, conn net.Conn) error {

	// Convert to SSH connection
	sshConn, channels, requests, err := ssh.NewServerConn(conn, s.config.sshConfig)
	if err != nil {
		s.config.Logger.Warn("SSH handshake failed:", "addr", conn.RemoteAddr().String(), "error", err)
		conn.Close()
		return err
	}

	// Close connection on exit
	s.config.Logger.Debug("Handshake successful")
	defer sshConn.Close()
	defer sshConn.Wait()

	// Discard requests
	go ssh.DiscardRequests(requests)

	// Create new tomb stone
	var t tomb.Tomb

	t.Go(func() error {
	OUTER:
		for {
			select {
			case ch := <-channels:

				// Check if chan was closed
				if ch == nil {
					t.Kill(nil)
					break OUTER
				}

				// Get channel type
				chType := ch.ChannelType()

				// Determine if channel is acceptable (has a registered handler)
				handler, ok := s.config.Handler(chType)
				if !ok {
					s.config.Logger.Info("UnknownChannelType", "type", chType)
					ch.Reject(ssh.UnknownChannelType, chType)
					t.Kill(nil)
					break OUTER
				}

				// Accept channel
				channel, requests, err := ch.Accept()
				if err != nil {
					s.config.Logger.Warn("Error creating channel")
					continue
				}

				t.Go(func() error {
					err := handler.Handle(t, sshConn, channel, requests)
					if err != nil {
						s.config.Logger.Warn("Handler raised an error", err)
					}
					s.config.Logger.Warn("Exiting channel", chType)
					t.Kill(err)
					return err
				})
			case <-parentTomb.Dying():
				t.Kill(nil)
				break OUTER
			case <-t.Dying():
				break OUTER
			}
		}
		return nil
	})

	// Wait for all goroutines to finish
	return t.Wait()
}
