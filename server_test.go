package smtpd_test

import (
	"fmt"
	"math/rand"
	"net/smtp"
	"strings"
	"testing"
	"time"

	"github.com/mailsac/smtpd"
)

type MessageRecorder struct {
	Messages []*smtpd.Message
}

func (m *MessageRecorder) Record(msg *smtpd.Message) error {
	m.Messages = append(m.Messages, msg)
	return nil
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func TestSMTPServer(t *testing.T) {

	recorder := &MessageRecorder{}
	server := smtpd.NewServer(recorder.Record)
	go server.ListenAndServe("localhost:0")
	defer server.Close()

	WaitUntilAlive(server)

	// Connect to the remote SMTP server.
	c, err := smtp.Dial(server.Address())
	if err != nil {
		t.Errorf("Should be able to dial localhost: %v", err)
	}

	// Set the sender and recipient first
	if err := c.Mail("sender@example.org"); err != nil {
		t.Errorf("Should be able to set a sender: %v", err)
	}
	if err := c.Rcpt("recipient@example.net"); err != nil {
		t.Errorf("Should be able to set a RCPT: %v", err)
	}

	if err := c.Rcpt("bcc@example.net"); err != nil {
		t.Errorf("Should be able to set a second RCPT: %v", err)
	}

	// Send the email body.
	wc, err := c.Data()
	if err != nil {
		t.Errorf("Error creating the data body: %v", err)
	}

	var emailBody = "This is the email body"

	_, err = fmt.Fprintf(wc, `From: sender@example.org
To: recipient@example.net
Content-Type: text/html

%v`, emailBody)
	if err != nil {
		t.Errorf("Error writing email: %v", err)
	}

	if err := wc.Close(); err != nil {
		t.Error(err)
	}

	// Send the QUIT command and close the connection.
	if err := c.Quit(); err != nil {
		t.Errorf("Server wouldn't accept QUIT: %v", err)
	}

	if len(recorder.Messages) != 1 {
		t.Fatalf("Expected 1 message, got: %v", len(recorder.Messages))
	}

	if h, err := recorder.Messages[0].HTML(); err == nil {
		if string(h) != emailBody {
			t.Errorf("Wrong body - want: %v, got: %v", emailBody, string(h))
		}
	} else {
		t.Fatalf("Error getting HTML body: %v", err)
	}

	bcc := recorder.Messages[0].BCC()
	if len(bcc) != 1 {
		t.Fatalf("Expected 1 BCC, got: %v", len(bcc))
	}

	if bcc[0].Address != "bcc@example.net" {
		t.Errorf("wrong BCC value, want: bcc@example.net, got: %v", bcc[0].Address)
	}

}

func TestSMTPServerLargeMessage(t *testing.T) {
	// sends message that is over the allowed length. Expects "connection reset by peer" from server

	recorder := &MessageRecorder{}
	server := smtpd.NewServer(recorder.Record)
	go server.ListenAndServe("localhost:0")
	defer server.Close()

	WaitUntilAlive(server)

	// Connect to the remote SMTP server.
	c, err := smtp.Dial(server.Address())
	if err != nil {
		t.Errorf("Should be able to dial localhost: %v", err)
	}

	// Set the sender and recipient first
	if err := c.Mail("sender@example.org"); err != nil {
		t.Errorf("Should be able to set a sender: %v", err)
	}
	if err := c.Rcpt("recipient@example.net"); err != nil {
		t.Errorf("Should be able to set a RCPT: %v", err)
	}

	// Send the email body.
	wc, err := c.Data()
	if err != nil {
		t.Errorf("Error creating the data body: %v", err)
	}

	var bodySizeKB = 500
	var bodySize = bodySizeKB * 1024
	var emailBody = "This is the email body" + RandStringBytes(bodySize) + "\n.\n"

	written, err := fmt.Fprintf(wc, `From: sender@example.org
To: recipient@example.net
Content-Type: text/html

%v`, emailBody)
	t.Log("written bytes", written, "/", 100+bodySize)

	var expected = "broken pipe"
	var actual string
	if err != nil {
		actual = err.Error()
	}
	if !strings.Contains(actual, expected) {
		t.Errorf("Error actual = %v, and Expected error to contain '%v'.", actual, expected)
	}
}

func TestSMTPServerTimeout(t *testing.T) {

	recorder := &MessageRecorder{}
	server := smtpd.NewServer(recorder.Record)

	// Set some really short timeouts
	server.ReadTimeout = time.Millisecond * 1
	server.WriteTimeout = time.Millisecond * 1

	go server.ListenAndServe("localhost:0")
	defer server.Close()

	WaitUntilAlive(server)

	// Connect to the remote SMTP server.
	c, err := smtp.Dial(server.Address())
	if err != nil {
		t.Errorf("Should be able to dial localhost: %v", err)
	}

	// Sleep for twice the timeout
	time.Sleep(time.Millisecond * 20)

	// Set the sender and recipient first
	if err := c.Hello("sender@example.org"); err == nil {
		t.Errorf("Should have gotten a timeout from the upstream server")
	}

}

func TestSMTPServerNoTLS(t *testing.T) {

	recorder := &MessageRecorder{}
	server := smtpd.NewServer(recorder.Record)

	go server.ListenAndServe("localhost:0")
	defer server.Close()

	WaitUntilAlive(server)

	// Connect to the remote SMTP server.
	c, err := smtp.Dial(server.Address())
	if err != nil {
		t.Errorf("Should be able to dial localhost: %v", err)
	}

	err = c.StartTLS(nil)
	if err == nil {
		t.Error("Server should return a failure for a TLS request when there is no config available")
	}

}

func TestSMTPServerNoAuthCustomVerb(t *testing.T) {

	fakeAuthHandler := func(email, apiKey string) (acct smtpd.AuthUser, passed bool) {
		return nil, false
	}
	setup := func() (*smtpd.Server, *smtp.Client) {
		recorder := &MessageRecorder{}
		server := smtpd.NewServer(recorder.Record)
		serverAuth := smtpd.NewAuth()
		serverAuth.Extend("PLAIN", &smtpd.AuthPlain{Auth: fakeAuthHandler})

		server.Auth = serverAuth

		go server.ListenAndServe("localhost:0")

		WaitUntilAlive(server)

		// Connect to the remote SMTP server.
		c, err := smtp.Dial(server.Address())
		if err != nil {
			t.Errorf("Should be able to dial localhost: %v", err)
		}

		return server, c
	}

	t.Run("prevents verb when NOT in pre auth verbs", func(t *testing.T) {
		server, c := setup()
		defer server.Close()

		// remove support for any methods
		// first ie HELO
		server.PreAuthVerbsAllowed = []string{"AUTH", "FAKE"}

		// check support
		err := c.Hello("you.io")
		if err == nil {
			t.Errorf("Should have NOT allowed HELO")
		}
	})
	t.Run("allows extension verb when IS included as pre auth ok", func(t *testing.T) {
		server, c := setup()
		defer server.Close()

		// the test change
		server.PreAuthVerbsAllowed = []string{"AUTH", "HELO"}
		err := c.Hello("me.com")
		if err != nil {
			t.Errorf("Should have allowed HELO, %v", err)
		}
	})
}
