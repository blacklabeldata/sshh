package ssh

import (
	"io"
	"net"

	"golang.org/x/crypto/ssh"

	"github.com/stretchr/testify/mock"
)

// MockNewChannel mocks an ssh.NewChannel.
type MockNewChannel struct {
	mock.Mock
	TypeName       string
	Channel        ssh.Channel
	AcceptErr      error
	RejectErr      error
	RequestChannel <-chan *ssh.Request
	ExtData        []byte
}

// Accept accepts the channel creation request. It returns the Channel
// and a Go channel containing SSH requests. The Go channel must be
// serviced otherwise the Channel will hang.
func (m *MockNewChannel) Accept() (ssh.Channel, <-chan *ssh.Request, error) {
	m.Called()
	return m.Channel, m.RequestChannel, m.AcceptErr
}

// Reject rejects the channel creation request. After calling
// this, no other methods on the Channel may be called.
func (m *MockNewChannel) Reject(reason ssh.RejectionReason, message string) error {
	m.Called(reason, message)
	return m.RejectErr
}

// ChannelType returns the type of the channel, as supplied by the
// client.
func (m *MockNewChannel) ChannelType() string {
	m.Called()
	return m.TypeName
}

// ExtraData returns the arbitrary payload for this channel, as supplied
// by the client. This data is specific to the channel type.
func (m *MockNewChannel) ExtraData() []byte {
	m.Called()
	return m.ExtData
}

// MockChannel mocks an ssh.Channel
type MockChannel struct {
	mock.Mock
	ReadError        error
	WriteError       error
	CloseError       error
	SendSuccess      bool
	SendError        error
	StderrReadWriter io.ReadWriter
}

// Read reads up to len(data) bytes from the channel.
func (m *MockChannel) Read(data []byte) (int, error) {
	m.Called(data)

	// Mock read failure
	if m.ReadError != nil {
		return 0, m.ReadError
	}

	// Mock successful read
	return len(data), nil
}

// Write writes len(data) bytes to the channel.
func (m *MockChannel) Write(data []byte) (int, error) {
	m.Called(data)

	// Failed write
	if m.WriteError != nil {
		return 0, m.WriteError
	}

	// Successful write
	return len(data), nil
}

// Close signals end of channel use. No data may be sent after this
// call.
func (m *MockChannel) Close() error {
	m.Called()
	return m.CloseError
}

// CloseWrite signals the end of sending in-band
// data. Requests may still be sent, and the other side may
// still send data
func (m *MockChannel) CloseWrite() error {
	m.Called()
	return m.CloseError
}

// SendRequest sends a channel request.  If wantReply is true,
// it will wait for a reply and return the result as a
// boolean, otherwise the return value will be false. Channel
// requests are out-of-band messages so they may be sent even
// if the data stream is closed or blocked by flow control.
func (m *MockChannel) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	m.Called(name, wantReply, payload)
	return m.SendSuccess, m.SendError
}

// Stderr returns an io.ReadWriter that writes to this channel
// with the extended data type set to stderr. Stderr may
// safely be read and written from a different goroutine than
// Read and Write respectively.
func (m *MockChannel) Stderr() io.ReadWriter {
	m.Called()
	return m.StderrReadWriter
}

// MockConnMetadata mocks ssh.ConnMetadata
type MockConnMetadata struct {
	mock.Mock
	UserName    string
	SessionData []byte
	ClientVer   []byte
	ServerVer   []byte
	Remote      net.Addr
	Local       net.Addr
}

// User returns the user ID for this connection.
// It is empty if no authentication is used.
func (m *MockConnMetadata) User() string {
	m.Called()
	return m.UserName
}

// SessionID returns the sesson hash, also denoted by H.
func (m *MockConnMetadata) SessionID() []byte {
	m.Called()
	return m.SessionData
}

// ClientVersion returns the client's version string as hashed
// into the session ID.
func (m *MockConnMetadata) ClientVersion() []byte {
	m.Called()
	return m.ClientVer
}

// ServerVersion returns the server's version string as hashed
// into the session ID.
func (m *MockConnMetadata) ServerVersion() []byte {
	m.Called()
	return m.ServerVer
}

// RemoteAddr returns the remote address for this connection.
func (m *MockConnMetadata) RemoteAddr() net.Addr {
	m.Called()
	return m.Remote
}

// LocalAddr returns the local address for this connection.
func (m *MockConnMetadata) LocalAddr() net.Addr {
	m.Called()
	return m.Local
}

// MockConn mocks ssh.Conn
type MockConn struct {
	MockConnMetadata
	RequestSuccess bool
	RequestData    []byte
	RequestError   error
	Channel        ssh.Channel
	Requests       <-chan *ssh.Request
	OpenError      error
	CloseError     error
	WaitError      error
}

// SendRequest sends a global request, and returns the
// reply. If wantReply is true, it returns the response status
// and payload. See also RFC4254, section 4.
func (m *MockConn) SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error) {
	m.Called(name, wantReply, payload)
	return m.RequestSuccess, m.RequestData, m.RequestError
}

// OpenChannel tries to open an channel. If the request is
// rejected, it returns *OpenChannelError. On success it returns
// the SSH Channel and a Go channel for incoming, out-of-band
// requests. The Go channel must be serviced, or the
// connection will hang.
func (m *MockConn) OpenChannel(name string, data []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	m.Called(name, data)
	return m.Channel, m.Requests, m.OpenError
}

// Close closes the underlying network connection
func (m *MockConn) Close() error {
	m.Called()
	return m.CloseError
}

// Wait blocks until the connection has shut down, and returns the
// error causing the shutdown.
func (m *MockConn) Wait() error {
	m.Called()
	return m.WaitError
}
