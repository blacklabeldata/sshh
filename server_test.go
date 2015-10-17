package sshh

import (
	"errors"
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
	_, _, err = client.OpenChannel("shell", []byte{})
	suite.NotNil(err, "server should not accept shell channels")
	suite.T().Logf(err.Error())
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
