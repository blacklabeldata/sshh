package sshh

import (
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/blacklabeldata/grim"
	sshmocks "github.com/blacklabeldata/mockery/ssh"
	"github.com/blacklabeldata/sshh/router"
	log "github.com/mgutz/logxi/v1"

	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
)

// TestUserTestSuite runs the UserTestSuite
func TestServerSuite(t *testing.T) {
	suite.Run(t, new(ServerSuite))
}

// ServerSuite tests SSH server
type ServerSuite struct {
	suite.Suite
	server *SSHServer
}

func (suite *ServerSuite) createConfig() Config {

	// Create logger
	writer := log.NewConcurrentWriter(os.Stdout)
	// writer := log.NewConcurrentWriter(ioutil.Discard)
	logger := log.NewLogger(writer, "sshh")
	// logger := log.DefaultLog

	// Get signer
	signer, err := ssh.ParsePrivateKey([]byte(serverKey))
	if err != nil {
		suite.Fail("Private key could not be parsed", err.Error())
	}

	r := router.New(logger, nil, nil)
	r.Register("/echo", &EchoHandler{log.New("echo")})
	r.Register("/bad", &BadHandler{})

	// Create config
	cfg := Config{
		Context:  context.Background(),
		Deadline: time.Second,
		Router:   r,
		// Handlers: map[string]SSHHandler{
		// 	"echo": &EchoHandler{log.New("echo")},
		// 	"bad":  &BadHandler{},
		// },
		Logger:            logger,
		Bind:              ":9022",
		PrivateKey:        signer,
		PasswordCallback:  passwordCallback,
		PublicKeyCallback: publicKeyCallback,
	}
	return cfg
}

// SetupTest prepares the suite before a test is ran.
func (suite *ServerSuite) SetupTest() {

	cfg := suite.createConfig()
	server, err := New(&cfg)
	if err != nil {
		suite.Fail("error creating server: " + err.Error())
	}
	suite.server = &server
	suite.server.Start()
}

// TearDownSuite cleans up suite state after all the tests have completed.
func (suite *ServerSuite) TearDownTest() {
	suite.server.Stop()
}

func (suite *ServerSuite) TestClientConnection() {

	// Get signer
	signer, err := ssh.ParsePrivateKey([]byte(clientPrivateKey))
	if err != nil {
		suite.Fail("Private key could not be parsed" + err.Error())
	}

	// Configure client connection
	config := &ssh.ClientConfig{
		User: "admin",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}

	// Create client connection
	client, err := ssh.Dial("tcp", "127.0.0.1:9022", config)
	if err != nil {
		suite.Fail(err.Error())
		return
	}
	defer client.Close()

	// Open channel
	channel, requests, err := client.OpenChannel("/echo", []byte{})
	if err != nil {
		suite.Fail(err.Error())
		return
	}
	go ssh.DiscardRequests(requests)
	defer channel.Close()
}

func (suite *ServerSuite) TestUnknownChannel() {

	// Get signer
	signer, err := ssh.ParsePrivateKey([]byte(clientPrivateKey))
	if err != nil {
		suite.Fail("Private key could not be parsed" + err.Error())
	}

	// Configure client connection
	config := &ssh.ClientConfig{
		User: "admin",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}

	// Create client connection
	client, err := ssh.Dial("tcp", "127.0.0.1:9022", config)
	if err != nil {
		suite.Fail(err.Error())
		return
	}
	defer client.Close()

	// Open channel
	_, _, err = client.OpenChannel("/shell", []byte{})
	suite.NotNil(err, "server should not accept shell channels")
}

func (suite *ServerSuite) TestHandlerError() {

	// Configure client connection
	config := &ssh.ClientConfig{
		User: "jonny.quest",
		Auth: []ssh.AuthMethod{
			ssh.Password("bandit"),
		},
	}

	// Create client connection
	client, err := ssh.Dial("tcp", "127.0.0.1:9022", config)
	if err != nil {
		suite.Fail(err.Error())
		return
	}
	defer client.Close()

	// Open channel
	channel, requests, err := client.OpenChannel("/bad", []byte{})
	if err != nil {
		suite.Fail(err.Error())
		return
	}
	go ssh.DiscardRequests(requests)
	defer channel.Close()
}

func (suite *ServerSuite) TestUnacceptableChannel() {
	g := grim.Reaper()

	r := router.New(log.NullLog, nil, nil)
	r.Register("/echo", &EchoHandler{log.New("echo")})
	r.Register("/bad", &BadHandler{})

	acceptErr := errors.New("accept error")
	ch := &sshmocks.MockNewChannel{
		TypeName:  "/echo",
		AcceptErr: acceptErr,
	}
	ch.On("ChannelType").Return("/echo")
	ch.On("Accept").Return(nil, nil, acceptErr)
	ch.On("Reject", ChannelAcceptError, "/echo").Return(errors.New("unknown reason 1000"))

	conn := &sshmocks.MockConn{}
	conn.On("Close").Return(nil)
	serverConn := ssh.ServerConn{
		Conn: conn,
	}
	g.SpawnFunc(channelHandler(g, log.NullLog, &serverConn, ch, r))
	g.Wait()

	// assert that the expectations were met
	ch.AssertCalled(suite.T(), "ChannelType")
	ch.AssertCalled(suite.T(), "Accept")
	ch.AssertCalled(suite.T(), "Reject", ChannelAcceptError, "/echo")
	conn.AssertCalled(suite.T(), "Close")
}

func (suite *ServerSuite) TestInvalidChannelType() {
	g := grim.Reaper()

	r := router.New(log.NullLog, nil, nil)
	r.Register("/echo", &EchoHandler{log.New("echo")})
	r.Register("/bad", &BadHandler{})

	acceptErr := errors.New("accept error")
	ch := &sshmocks.MockNewChannel{
		TypeName:  ":/route",
		AcceptErr: acceptErr,
	}
	ch.On("ChannelType").Return(":/route")
	ch.On("Reject", InvalidChannelType, "invalid channel URI").Return(errors.New("unknown reason 1001"))

	conn := &sshmocks.MockConn{}
	conn.On("Close").Return(nil)
	serverConn := ssh.ServerConn{
		Conn: conn,
	}
	g.SpawnFunc(channelHandler(g, log.NullLog, &serverConn, ch, r))
	g.Wait()

	// assert that the expectations were met
	ch.AssertCalled(suite.T(), "ChannelType")
	ch.AssertCalled(suite.T(), "Reject", InvalidChannelType, "invalid channel URI")
	conn.AssertCalled(suite.T(), "Close")
}

func (suite *ServerSuite) TestSchemeNotSupported() {
	g := grim.Reaper()

	r := router.New(log.NullLog, nil, nil)
	r.Register("/echo", &EchoHandler{log.New("echo")})
	r.Register("/bad", &BadHandler{})

	acceptErr := errors.New("accept error")
	ch := &sshmocks.MockNewChannel{
		TypeName:  "https://user@example.com/api/route",
		AcceptErr: acceptErr,
	}
	ch.On("ChannelType").Return("https://user@example.com/api/route")
	ch.On("Reject", SchemeNotSupported, "schemes are not supported in the channel URI").Return(errors.New("unknown reason 1002"))

	conn := &sshmocks.MockConn{}
	conn.On("Close").Return(nil)
	serverConn := ssh.ServerConn{
		Conn: conn,
	}
	g.SpawnFunc(channelHandler(g, log.NullLog, &serverConn, ch, r))
	g.Wait()

	// assert that the expectations were met
	ch.AssertCalled(suite.T(), "ChannelType")
	ch.AssertCalled(suite.T(), "Reject", SchemeNotSupported, "schemes are not supported in the channel URI")
	conn.AssertCalled(suite.T(), "Close")
}

func (suite *ServerSuite) TestUserNotSupported() {
	channel := "user@example.com/echo"
	ch := &sshmocks.MockNewChannel{}
	ch.On("Reject", UserNotSupported, "users are not supported in the channel URI").Return(errors.New("unknown reason 1005"))

	uri := url.URL{
		User: url.User("user"),
	}
	rejected := reject(channel, &uri, ch, log.NullLog)
	suite.True(rejected, "channel should have been rejected")

	// assert that the expectations were met
	ch.AssertCalled(suite.T(), "Reject", UserNotSupported, "users are not supported in the channel URI")
}

func (suite *ServerSuite) TestHostNotSupported() {
	channel := "user@example.com/echo"
	ch := &sshmocks.MockNewChannel{}
	ch.On("Reject", HostNotSupported, "hosts are not supported in the channel URI").Return(errors.New("unknown reason 1004"))

	uri := url.URL{
		Host: "example.com",
	}
	rejected := reject(channel, &uri, ch, log.NullLog)
	suite.True(rejected, "channel should have been rejected")

	// assert that the expectations were met
	ch.AssertCalled(suite.T(), "Reject", HostNotSupported, "hosts are not supported in the channel URI")
}

func (suite *ServerSuite) TestInvalidQueryParams() {
	g := grim.Reaper()

	channel := "/echo?%"
	r := router.New(log.NullLog, nil, nil)
	r.Register("/echo", &EchoHandler{log.New("echo")})
	r.Register("/bad", &BadHandler{})

	acceptErr := errors.New("accept error")
	ch := &sshmocks.MockNewChannel{
		TypeName:  channel,
		AcceptErr: acceptErr,
	}
	ch.On("ChannelType").Return(channel)
	ch.On("Reject", InvalidQueryParams, "invalid query params in channel type").Return(errors.New("unknown reason 1002"))

	conn := &sshmocks.MockConn{}
	conn.On("Close").Return(nil)
	serverConn := ssh.ServerConn{
		Conn: conn,
	}
	g.SpawnFunc(channelHandler(g, log.NullLog, &serverConn, ch, r))
	g.Wait()

	// assert that the expectations were met
	ch.AssertCalled(suite.T(), "ChannelType")
	ch.AssertCalled(suite.T(), "Reject", InvalidQueryParams, "invalid query params in channel type")
	conn.AssertCalled(suite.T(), "Close")
}

func (suite *ServerSuite) TestChannelHandleError() {
	g := grim.Reaper()

	channel := "/bad"
	r := router.New(log.NullLog, nil, nil)
	r.Register("/bad", &BadHandler{})

	c := &sshmocks.MockChannel{}
	c.On("Close").Return(nil)

	ch := &sshmocks.MockNewChannel{
		TypeName: channel,
		Channel:  c,
	}
	ch.On("ChannelType").Return(channel)
	ch.On("Accept").Return(c, nil, nil)
	ch.On("Reject", ChannelHandleError, "error handling channel: an error occurred").
		Return(errors.New("error handling channel: an error occurred"))

	conn := &sshmocks.MockConn{}
	conn.On("Close").Return(nil)
	serverConn := ssh.ServerConn{
		Conn: conn,
	}
	g.SpawnFunc(channelHandler(g, log.NullLog, &serverConn, ch, r))
	g.Wait()

	// assert that the expectations were met
	ch.AssertCalled(suite.T(), "ChannelType")
	ch.AssertCalled(suite.T(), "Accept")
	ch.AssertCalled(suite.T(), "Reject", ChannelHandleError, "error handling channel: an error occurred")
	c.AssertCalled(suite.T(), "Close")
	conn.AssertCalled(suite.T(), "Close")
}

func (suite *ServerSuite) TestWildcard() {
	g := grim.Reaper()

	writer := log.NewConcurrentWriter(os.Stdout)
	logger := log.NewLogger(writer, "sshh_test")

	r := router.New(logger, nil, nil)
	r.Register("/echo", &EchoHandler{log.New("echo")})
	r.Register("/bad", &BadHandler{})

	acceptErr := errors.New("accept error")
	ch := &sshmocks.MockNewChannel{
		TypeName:  "*",
		AcceptErr: acceptErr,
	}
	ch.On("ChannelType").Return("*")
	ch.On("Reject", ssh.UnknownChannelType, "*").Return(errors.New("unknown reason 1000"))

	conn := &sshmocks.MockConn{}
	conn.On("Close").Return(nil)
	serverConn := ssh.ServerConn{
		Conn: conn,
	}
	g.SpawnFunc(channelHandler(g, logger, &serverConn, ch, r))
	g.Wait()

	// assert that the expectations were met
	ch.AssertCalled(suite.T(), "ChannelType")
	ch.AssertCalled(suite.T(), "Reject", ssh.UnknownChannelType, "*")
	conn.AssertCalled(suite.T(), "Close")
}

func (suite *ServerSuite) TestShell() {
	g := grim.Reaper()

	writer := log.NewConcurrentWriter(os.Stdout)
	logger := log.NewLogger(writer, "sshh_test")

	r := router.New(logger, nil, nil)
	r.Register("shell", &EchoHandler{log.New("echo")})

	acceptErr := errors.New("accept error")
	ch := &sshmocks.MockNewChannel{
		TypeName:  "shell",
		AcceptErr: acceptErr,
	}
	ch.On("ChannelType").Return("shell")
	ch.On("Reject", ssh.UnknownChannelType, "shell").Return(errors.New("unknown reason 1000"))

	conn := &sshmocks.MockConn{}
	conn.On("Close").Return(nil)
	serverConn := ssh.ServerConn{
		Conn: conn,
	}
	g.SpawnFunc(channelHandler(g, logger, &serverConn, ch, r))
	g.Wait()

	// assert that the expectations were met
	ch.AssertCalled(suite.T(), "ChannelType")
	ch.AssertCalled(suite.T(), "Reject", ssh.UnknownChannelType, "shell")
	conn.AssertCalled(suite.T(), "Close")
}
