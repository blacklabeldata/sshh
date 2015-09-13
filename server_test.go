package sshh

import (
	"os"
	"testing"
	"time"

	log "github.com/mgutz/logxi/v1"

	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/ssh"
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

	// Create config
	cfg := Config{
		Deadline: time.Second,
		Handlers: map[string]SSHHandler{
			"echo": &EchoHandler{log.New("echo")},
			"bad":  &BadHandler{},
		},
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
	server, err := NewSSHServer(&cfg)
	if err != nil {
		suite.Fail("error creating server: " + err.Error())
	}
	suite.server = &server
	suite.server.Start()
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
	channel, requests, err := client.OpenChannel("echo", []byte{})
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
	channel, requests, err := client.OpenChannel("bad", []byte{})
	if err != nil {
		suite.Fail(err.Error())
		return
	}
	go ssh.DiscardRequests(requests)
	defer channel.Close()
}

// TearDownSuite cleans up suite state after all the tests have completed.
func (suite *ServerSuite) TearDownTest() {
	suite.server.Stop()
}
