package smtpd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/mail"
	"net/textproto"
	"strings"
	"sync"
	"time"
)

// LimitedReader keeps from reading past the suggested max size. It was copied from io and
// altered to return a SMTPError.
type LimitedReader struct {
	R              io.Reader // underlying reader
	N              int64     // max bytes remaining
	ReadsRemaining int
	DidHitLimit    bool
}

func (l *LimitedReader) Read(p []byte) (n int, err error) {
	if l.N <= 0 && !l.DidHitLimit {
		l.DidHitLimit = true
		l.ReadsRemaining = 10 // allow filling the buffer
	}
	if l.DidHitLimit {
		l.ReadsRemaining--
		// it will still Read a few more times as TextProto fills the buffer
		// before responding with the error
		err = SMTPError{552, errors.New("Message size too large")}
		if l.ReadsRemaining <= 0 {
			// bufio builtin needs regular error. we will already have written 552 to smtp by
			// the time this code path is traveled.
			return 0, err
		}
	}

	n, rerr := l.R.Read(p)
	if err != nil && rerr != nil {
		err = rerr
	}
	l.N -= int64(n)
	return n, err
}

// Conn is a wrapper for net.Conn that provides
// convenience handlers for SMTP requests
type Conn struct {
	// ID is this connection ID which changes after TLS connection
	ID string
	// optional hostname given during NAME
	ClientHostname string
	// Conn is primarily a wrapper around a net.Conn object
	net.Conn

	ForwardedForIP string

	// Track some mutable for this connection
	IsTLS    bool
	Errors   []error
	User     AuthUser
	FromAddr *mail.Address
	ToAddr   []*mail.Address
	// any additional text information here, like custom headers you will later prepend when passing along to another server
	AdditionalHeaders string

	// Configuration options
	MaxSize      int64
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// internal state
	lock        sync.Mutex
	transaction int

	asTextProto sync.Once
	textProto   *textproto.Conn

	Logger *log.Logger

	server *Server
}

// AddInfoHeader adds an additional header to the beginning of the list, such that the newest
// headers will be at the top
func (c *Conn) AddInfoHeader(headerName, headerText string) {
	c.AdditionalHeaders = headerName + ": " + headerText + "\n" + c.AdditionalHeaders
}

// tp returns a textproto wrapper for this connection
func (c *Conn) tp() *textproto.Conn {
	c.asTextProto.Do(func() {
		c.setupTextProto()
	})
	return c.textProto
}

func (c *Conn) setupTextProto() {
	c.textProto = textproto.NewConn(c)
	if c.MaxSize > 0 {
		c.textProto.Reader = *textproto.NewReader(bufio.NewReader(&LimitedReader{c, c.MaxSize, 0, false}))
	}
}

// StartTX starts a new MAIL transaction
func (c *Conn) StartTX(from *mail.Address) error {
	if c.transaction != 0 {
		return ErrTransaction
	}
	c.transaction = int(time.Now().UnixNano())
	c.FromAddr = from
	return nil
}

// EndTX closes off a MAIL transaction and returns a message object
func (c *Conn) EndTX() error {
	if c.transaction == 0 {
		return ErrTransaction
	}
	c.transaction = 0
	return nil
}

func (c *Conn) Reset() {
	c.ResetBuffers()
	c.User = nil
	c.resetTextProto() // reset LimitedReader
	if c.server.Verbose {
		c.Logger.Println(c.ID, "SERVER: resetting connection")
	}
}

// ResetBuffers resets the mail buffers (to, from)
func (c *Conn) ResetBuffers() {
	c.FromAddr = nil
	c.ToAddr = make([]*mail.Address, 0)
	c.AdditionalHeaders = ""
	c.transaction = 0
	c.setupTextProto()
}

// ReadSMTP pulls a single SMTP command line (ending in a carriage return + newline)
func (c *Conn) ReadSMTP() (string, string, error) {
	c.SetReadDeadline(time.Now().Add(c.ReadTimeout))
	if line, err := c.tp().ReadLine(); err == nil {
		var args string
		command := strings.SplitN(line, " ", 2)

		verb := strings.ToUpper(command[0])
		if len(command) > 1 {
			args = command[1]
		}

		return verb, args, nil
	} else {
		return "", "", err
	}
}

// ReadLine reads a single line from the client
func (c *Conn) ReadLine() (string, error) {
	c.SetReadDeadline(time.Now().Add(c.ReadTimeout))
	return c.tp().ReadLine()
}

// ReadData brokers the special case of SMTP data messages
func (c *Conn) ReadData() (string, error) {
	c.SetReadDeadline(time.Now().Add(c.ReadTimeout))
	lines, err := c.tp().ReadDotLines()
	return strings.Join(lines, "\n"), err
}

// WriteSMTP writes a general SMTP line
func (c *Conn) WriteSMTP(code int, message string) error {
	c.SetWriteDeadline(time.Now().Add(c.WriteTimeout))
	msg := fmt.Sprintf("%v %v", code, message) + "\r\n"
	_, err := c.Write([]byte(msg))
	if c.server.Verbose {
		c.Logger.Println(c.ID, " SERVER: ", msg)
	}
	return err
}

// WriteEHLO writes an EHLO line, see https://tools.ietf.org/html/rfc2821#section-4.1.1.1
func (c *Conn) WriteEHLO(message string) error {
	c.SetWriteDeadline(time.Now().Add(c.WriteTimeout))
	msg := fmt.Sprintf("250-%v", message) + "\r\n"
	_, err := c.Write([]byte(msg))
	if c.server.Verbose {
		c.Logger.Println(c.ID, " SERVER: ", msg)
	}
	return err
}

const OK string = "OK"

// WriteOK is a convenience function for sending the default OK response
func (c *Conn) WriteOK() error {
	if c.server.Verbose {
		c.Logger.Println(c.ID, " SERVER: ", 250, OK)
	}
	return c.WriteSMTP(250, OK)
}
