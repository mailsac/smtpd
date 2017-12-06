package smtpd

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/mail"
	"net/textproto"
	"strings"
	"sync"
	"time"
)

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
		c.textProto = textproto.NewConn(c)
		if c.MaxSize > 0 {
			c.textProto.Reader = *textproto.NewReader(bufio.NewReader(io.LimitReader(c, c.MaxSize)))
		}
	})
	return c.textProto
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

// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-golang
var randSrc = rand.NewSource(time.Now().UnixNano())

const idLen = 8

func randStringBytesMaskImprSrc() string {
	b := make([]byte, idLen)
	// A randSrc.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := idLen-1, randSrc.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = randSrc.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}
