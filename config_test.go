package sshh

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	log "github.com/mgutz/logxi/v1"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
	tomb "gopkg.in/tomb.v2"
)

var serverKey = `
-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQDjzAhRGLLcnQhs7Xe/2TrbjpHOkeBwVfmI0z+mZot87AXyIVcr
+OepPl/8UekPb352bz3zAwn2x5zCT/hW+1CBwp6fqhAvlxlYFEYr40L2dYKMmZyT
3kq18P3fTmAIKyXv7XOtVXiNLHc0Ai+3aN4J+yHKwbf42nNU3Qb1NRp9KQIDAQAB
AoGANgZyxoD8EpRvph3fs7FaYy356KryNtI9HzUyuE1DsbnsYxODMBuVHa98ZkQq
6Q1BSedyIstKtqt6wx7iQAbUfa9VxYht2DnxJDG7AhbQS1jd8ifSPCyhsp7HqCL5
pPbJBoW2M2qVL95+TMaZKYDDQcpFIHsEzJ/6lnWatGdBxfECQQDwv+cFSe5i8hqU
5BmLH3131ez5jO4yCziQxNwZaEavDXPDsqeKl/8Oj9EOcVyysyOLR9z7NzOCV2wX
8u0hpO69AkEA8joVv2rZdb+83Zc1UF/qnihMt4ZqYafPMXEtl2YTZtDmQOZG0kMw
a/iPjkUt/t8+CNR/Z5RLUYA5NVJSlsI03QJBANUZaEo8KLCYkILebOXCl/Ks/zfd
UTIm0IkEV7Z9oKNuitvclYSOCgw/rNLV8TGUc4/jqm0LbaKf82Q3eULglRkCQBsi
4rjVEZOdbV0tyW09sZ0SSrXsuxJBqHaThVYGu3mzQXhX0+tOV6hg6kQ3/9Uj0WFP
3Q4PkPiKct5EYLg+/YkCQCpHiRgfbESG2J/eYtTdyDvm+r0m0pc4vitqKsRGjd2u
LZxh0eGWnXXd+Os/wOVMSzkAWuzc4VTxMUnk/yf13IA=
-----END RSA PRIVATE KEY-----
`
var clientPrivateKey = `
-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQC/sjgGi0ciXKeBQ3TClw+Vae22MF8wR4otOTCws2f/FF2aLOd6
eR+qjyXS/WbWKoh+kAgPUp2B1Gf+T6AMAefW6ZgGxdqgBg36XTFXZPD9X2BWUOaP
nlCqZ2z7RFvKBUimlb/OpSFjxFyP8Wq7cx6ehrTSzzi836Cu8TGWPgHz0QIDAQAB
AoGADcScF4Q7WLF06mjQ4wT8fou8IgC5ZXtN5k+cOqS4DG8HBgLBoV8/sf1UByJi
F3G4mfZ4TbluTJvX2EEZyqL8ZqhQDpmeH0IcmqBN7J8eowNAE6ufaJwk3t+FOdtc
6rEYGbr9uY0e9WZUE7C8Xh1t/ZeA0tsbonFhUStFxhN0vgECQQDdj82fsWwLChPq
y+tyaFq5Yx7KyJx+oWUBZ+6ycWgOEBTNZFDVuIuuVWzbI4IfmRNUDyN6W3aWeXtA
iuWRmHJhAkEA3X4HyIWBMo1L1FE/gsEU+edNnxOMkvWJy9OzEjocdVsdY6mB5coP
U+T7H2l+8+dGspjwU+nA7YYhw75+IqjXcQJAM+CE49xWEOumKDbhBSO8AmZcAl0g
j2HY1ZBxSmTVWV2YkVLovnH8erBT0aepwx5DcU4uH2slBCyjmEQtZn7MYQJBAI4e
JtpYJ10LYoN6GnlIcLAk5R5UCdfl6qO5U2Y3mTkH3KStB+csrocTHrq6EzZmyGsi
TNpa22rMrO+PVBnjIlECQG9zJBbgD/+geE0AcaNyaPW0/tG+LiYkkZBdmVRFuSGB
Fl62wKXrIdmZwFgITeyfEOUxV55Zv5DEg0MPPZsPmu0=
-----END RSA PRIVATE KEY-----
`

var clientPublicKey = `
-----BEGIN RSA PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC/sjgGi0ciXKeBQ3TClw+Vae22
MF8wR4otOTCws2f/FF2aLOd6eR+qjyXS/WbWKoh+kAgPUp2B1Gf+T6AMAefW6ZgG
xdqgBg36XTFXZPD9X2BWUOaPnlCqZ2z7RFvKBUimlb/OpSFjxFyP8Wq7cx6ehrTS
zzi836Cu8TGWPgHz0QIDAQAB
-----END RSA PUBLIC KEY-----
`

func passwordCallback(conn ssh.ConnMetadata, password []byte) (perm *ssh.Permissions, err error) {
	if conn.User() == "jonny.quest" && string(password) == "bandit" {

		// Add username to permissions
		perm = &ssh.Permissions{
			Extensions: map[string]string{
				"username": conn.User(),
			},
		}
	} else {
		err = fmt.Errorf("Invalid username or password")
	}
	return
}

func publicKeyCallback(conn ssh.ConnMetadata, key ssh.PublicKey) (perm *ssh.Permissions, err error) {

	// Get signer
	privKey, err := ssh.ParsePrivateKey([]byte(clientPrivateKey))
	if err != nil {
		fmt.Println(err)
		err = fmt.Errorf("Unauthorized")
		return
	}
	// fmt.Printf("%#v\n", key.Marshal())
	// fmt.Printf("%#v\n", privKey.PublicKey().Marshal())

	if bytes.Equal(privKey.PublicKey().Marshal(), key.Marshal()) {
		// Add pubkey and username to permissions
		perm = &ssh.Permissions{
			Extensions: map[string]string{
				"username": conn.User(),
			},
		}
	} else {
		err = fmt.Errorf("Unauthorized")
	}
	return
}

type EchoHandler struct {
	logger log.Logger
}

func (e *EchoHandler) Handle(t tomb.Tomb, conn *ssh.ServerConn, channel ssh.Channel, requests <-chan *ssh.Request) error {
	defer channel.Close()
	e.logger.Info("echo handle called!")

	// Create tomb for terminal goroutines
	var tmb tomb.Tomb

	type msg struct {
		line     []byte
		isPrefix bool
		err      error
	}

	in := make(chan msg)
	defer close(in)
	reader := bufio.NewReader(channel)
	tmb.Go(func() error {
		tmb.Go(func() error {
			for {
				line, pre, err := reader.ReadLine()
				if err != nil {
					tmb.Kill(nil)
					return nil
				}

				select {
				case in <- msg{line, pre, err}:
				case <-t.Dying():
					tmb.Kill(nil)
					return nil
				case <-tmb.Dying():
					return nil
				}
			}
		})

		tmb.Go(func() error {
			for {
				e.logger.Info("time: ", time.Now())
				select {
				case <-tmb.Dying():
					return nil
				case <-t.Dying():
					tmb.Kill(nil)
					return nil
				case m := <-in:
					if m.err != nil {
						tmb.Kill(m.err)
						return m.err
					}

					// Send echo
					channel.Write(m.line)
				}
			}
		})
		return nil
	})

	return tmb.Wait()
}

type BadHandler struct {
}

func (BadHandler) Handle(t tomb.Tomb, conn *ssh.ServerConn, channel ssh.Channel, requests <-chan *ssh.Request) error {
	defer channel.Close()
	return fmt.Errorf("An error occurred")
}

func TestConfig(t *testing.T) {
	var authLogCalled bool
	var authLogCallback = func(conn ssh.ConnMetadata, method string, err error) {
		authLogCalled = true
	}

	// Create logger
	writer := log.NewConcurrentWriter(ioutil.Discard)
	logger := log.NewLogger(writer, "sshh")

	// Get signer
	signer, err := ssh.ParsePrivateKey([]byte(serverKey))
	if err != nil {
		t.Fatalf("Private key could not be parsed", err.Error())
	}

	cfg := Config{
		Deadline: time.Second,
		Handlers: map[string]SSHHandler{
			"echo": &EchoHandler{log.New("echo")},
		},
		Logger:            logger,
		Bind:              ":9022",
		PrivateKey:        signer,
		AuthLogCallback:   authLogCallback,
		PasswordCallback:  passwordCallback,
		PublicKeyCallback: publicKeyCallback,
	}

	// Assertions
	assert.Equal(t, time.Second, cfg.Deadline, "Deadline should be 1s")
	assert.Equal(t, ":9022", cfg.Bind, "Bind should be :9022")

	// Create SSH config
	c := cfg.SSHConfig()
	assert.NotNil(t, c, "SSH config should not be nil")
	assert.Equal(t, passwordCallback, c.PasswordCallback, "PasswordCallback should use the one we passed in")
	assert.Equal(t, publicKeyCallback, c.PublicKeyCallback, "PublicKeyCallback should use the one we passed in")
	assert.Equal(t, authLogCallback, c.AuthLogCallback, "AuthLogCallback should use the one we passed in")

	// Test Handlers
	h, ok := cfg.Handler("echo")
	assert.True(t, ok, "Echo handler should be registered")
	assert.NotNil(t, h, "Echo handler should not be nil")

	h, ok = cfg.Handler("shell")
	assert.False(t, ok, "Shell handler should be registered")
	assert.Nil(t, h, "Shell handler should be nil")
}

func TestEmptyBindConfig(t *testing.T) {
	cfg := Config{
		Bind: "",
	}

	// Create new server
	_, err := NewSSHServer(&cfg)
	assert.NotNil(t, err, "Empty bind addr should cause an error")
}

func TestBadAddrConfig(t *testing.T) {
	cfg := Config{
		Bind: "9",
	}

	// Create new server
	_, err := NewSSHServer(&cfg)
	assert.NotNil(t, err, "Invalid addr should return an error")
}

func TestUnavailableAddrConfig(t *testing.T) {
	cfg := Config{
		Bind: "9.9.9.9:9999",
	}

	// Create new server
	_, err := NewSSHServer(&cfg)
	assert.NotNil(t, err, "Invalid addr should return an error")
}
