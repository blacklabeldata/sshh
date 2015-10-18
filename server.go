package sshh

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	log "github.com/mgutz/logxi/v1"

	"github.com/blacklabeldata/grim"
	"github.com/blacklabeldata/sshh/router"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
)

// NewSSHServer creates a new server with the given config. The server will call `cfg.SSHConfig()` to setup
// the server. If an error occurs it will be returned. If the Bind address is empty or invalid
// an error will be returned. If there is an error starting the TCP server, the error will be returned.
func New(cfg *Config) (server SSHServer, err error) {
	if cfg.Context == nil {
		return SSHServer{}, errors.New("Config has no context")
	}

	// Create ssh config for server
	sshConfig := cfg.SSHConfig()
	cfg.sshConfig = sshConfig

	// Validate the ssh bind addr
	if cfg.Bind == "" {
		err = fmt.Errorf("ssh server: Empty SSH bind address")
		return
	}

	// Open SSH socket listener
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
	server.reaper = grim.ReaperWithContext(cfg.Context)
	return
}

// SSHServer handles all the incoming connections as well as handler dispatch.
type SSHServer struct {
	config   *Config
	Addr     *net.TCPAddr
	listener *net.TCPListener
	reaper   grim.GrimReaper
}

// Start starts accepting client connections. This method is non-blocking.
func (s *SSHServer) Start() {
	s.config.Logger.Info("Starting SSH server", "addr", s.config.Bind)
	s.reaper.SpawnFunc(s.listen)
}

// Stop stops the server and kills all goroutines. This method is blocking.
func (s *SSHServer) Stop() {
	s.reaper.Kill()
	s.config.Logger.Info("Shutting down SSH server...")
	s.reaper.Wait()
}

// listen accepts new connections and handles the conversion from TCP to SSH connections.
func (s *SSHServer) listen(c context.Context) {
	defer s.listener.Close()

	for {
		// Accepts will only block for 1s
		s.listener.SetDeadline(time.Now().Add(s.config.Deadline))

		select {

		// Stop server on channel receive
		case <-c.Done():
			s.config.Logger.Debug("Context Completed")
			return
		default:

			// Accept new connection
			tcpConn, err := s.listener.Accept()
			if err != nil {
				if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
					s.config.Logger.Debug("Connection timeout...")
				} else {
					s.config.Logger.Warn("Connection failed", "error", err)
				}
				continue
			}

			// Handle connection
			s.config.Logger.Info("Successful TCP connection:", tcpConn.RemoteAddr().String())
			s.reaper.Spawn(&tcpHandler{
				logger:          s.config.Logger,
				conn:            tcpConn,
				config:          s.config.sshConfig,
				discardRequests: s.config.DiscardRequests,
				router:          s.config.Router,
			})
		}
	}
}

type tcpHandler struct {
	logger          log.Logger
	conn            net.Conn
	config          *ssh.ServerConfig
	router          *router.Router
	discardRequests bool
}

func (t *tcpHandler) Execute(c context.Context) {
	select {
	case <-c.Done():
		t.conn.Close()
		return
	default:
	}

	// Create reaper
	g := grim.ReaperWithContext(c)
	defer g.Wait()

	// Convert to SSH connection
	sshConn, channels, requests, err := ssh.NewServerConn(t.conn, t.config)
	if err != nil {
		t.logger.Warn("SSH handshake failed:", "addr", t.conn.RemoteAddr().String(), "error", err)
		t.conn.Close()
		g.Kill()
		return
	}

	// Close connection on exit
	t.logger.Debug("Handshake successful")
	defer sshConn.Close()
	defer sshConn.Wait()

	// Discard all out-of-channel requests
	go ssh.DiscardRequests(requests)

OUTER:
	for {
		select {
		case <-c.Done():
			break OUTER
		case <-g.Dead():
			break OUTER
		case ch := <-channels:

			// Check if chan was closed
			if ch == nil {
				break OUTER
			}

			// Handle the channel
			g.SpawnFunc(channelHandler(g, t.logger, sshConn, ch, t.router))
		}
	}

	g.Kill()
}

func channelHandler(g grim.GrimReaper, logger log.Logger, conn *ssh.ServerConn, ch ssh.NewChannel, r *router.Router) grim.TaskFunc {
	return func(c context.Context) {
		defer conn.Close()

		// Get channel type
		chType := ch.ChannelType()

		// Parse channel URI
		uri, err := url.ParseRequestURI(chType)
		if err != nil {
			logger.Warn("Error parsing channel type", "type", chType, "err", err)
			ch.Reject(InvalidChannelType, "invalid channel URI")
			return
		} else if reject(chType, uri, ch, logger) {
			return
		}
		chType = uri.Path

		// Parse query params
		values, err := url.ParseQuery(uri.RawQuery)
		if err != nil {
			logger.Warn("Error parsing query params", "values", values, "err", err)
			ch.Reject(InvalidQueryParams, "invalid query params in channel type")
			return
		}

		// Determine if channel is acceptable (has a registered handler)
		if !r.HasRoute(chType) {
			logger.Info("UnknownChannelType", "type", chType)
			ch.Reject(ssh.UnknownChannelType, chType)
			return
		}

		// Otherwise, accept the channel
		channel, requests, err := ch.Accept()
		if err != nil {
			logger.Warn("Error creating channel", "type", chType, "err", err)
			ch.Reject(ChannelAcceptError, chType)
			return
		}

		// Handle the channel
		err = r.Handle(&router.Context{
			Path:     uri.Path,
			Context:  c,
			Values:   values,
			Channel:  channel,
			Requests: requests,
		})
		if err != nil {
			logger.Warn("Error handling channel", "type", chType, "err", err)
			ch.Reject(ChannelHandleError, fmt.Sprintf("error handling channel: %s", err.Error()))
			return
		}
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
