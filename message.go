package smtpd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
)

// Message is a nicely packaged representation of the received message
type Message struct {
	Conn *Conn

	To      []*mail.Address
	From    *mail.Address
	Header  mail.Header
	Subject string
	RawBody []byte
	Source  []byte

	MessageID string
	Rcpt      []*mail.Address

	// meta info
	Logger *log.Logger
}

// Part represents a single part of the message
type Part struct {
	Header   textproto.MIMEHeader
	part     *multipart.Part
	Body     []byte
	Children []*Part
}

// BCC returns a list of addresses this message should be
func (m *Message) BCC() []*mail.Address {

	var inHeaders = make(map[string]struct{})
	for _, to := range m.To {
		inHeaders[to.Address] = struct{}{}
	}

	var bcc []*mail.Address
	for _, recipient := range m.Rcpt {
		if _, ok := inHeaders[recipient.Address]; !ok {
			bcc = append(bcc, recipient)
		}
	}

	return bcc
}

// Plain returns the text/plain content of the message, if any
func (m *Message) Plain() ([]byte, error) {
	return m.FindBody("text/plain")
}

// HTML returns the text/html content of the message, if any
func (m *Message) HTML() ([]byte, error) {
	return m.FindBody("text/html")
}

func findTypeInParts(contentType string, parts []*Part) *Part {
	for _, p := range parts {
		mediaType, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if err == nil && mediaType == contentType {
			return p
		}
	}
	return nil
}

// Attachments returns the list of attachments on this message
// XXX: this assumes that the only mimetype supporting attachments is multipart/mixed
// need to review https://en.wikipedia.org/wiki/MIME#Multipart_messages to ensure that is the case
func (m *Message) Attachments() ([]*Part, error) {
	mediaType, _, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}

	parts, err := m.Parts()
	if err != nil {
		return nil, err
	}

	var attachments []*Part
	if mediaType == "multipart/mixed" {
		for _, part := range parts {
			mediaType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
			if err != nil {
				return nil, err
			}
			if strings.HasPrefix(mediaType, "multipart/") {
				// XXX: any cases where this would still be an attachment?
				continue
			}
			attachments = append(attachments, part)
		}
	}
	return attachments, nil
}

// FindBody finds the first part of the message with the specified Content-Type
func (m *Message) FindBody(contentType string) ([]byte, error) {

	mediaType, _, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}

	parts, err := m.Parts()
	if err != nil {
		return nil, err
	}

	var alternatives []*Part
	switch mediaType {
	case contentType:
		if len(parts) > 0 {
			return parts[0].Body, nil
		}
		return nil, fmt.Errorf("%v found, but no data in body", contentType)
	case "multipart/alternative":
		alternatives = parts
	default:
		if alt := findTypeInParts("multipart/alternative", parts); alt != nil {
			alternatives = alt.Children
		}
	}

	if len(alternatives) == 0 {
		return nil, fmt.Errorf("No multipart/alternative section found, can't find %v", contentType)
	}

	part := findTypeInParts(contentType, alternatives)
	if part == nil {
		return nil, fmt.Errorf("No %v content found in multipart/alternative section", contentType)
	}

	return part.Body, nil
}

func readToPart(header textproto.MIMEHeader, content io.Reader) (*Part, error) {
	cte := strings.ToLower(header.Get("Content-Transfer-Encoding"))

	if cte == "quoted-printable" {
		content = quotedprintable.NewReader(content)
	}

	slurp, err := ioutil.ReadAll(content)
	if err != nil {
		return nil, err
	}

	if cte == "base64" {
		dst := make([]byte, base64.StdEncoding.DecodedLen(len(slurp)))
		decodedLen, err := base64.StdEncoding.Decode(dst, slurp)
		if err != nil {
			return nil, err
		}

		slurp = dst[:decodedLen]
	}
	return &Part{
		Header: header,
		Body:   slurp,
	}, nil
}

func parseContent(header textproto.MIMEHeader, content io.Reader) ([]*Part, error) {

	mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil && err.Error() == "mime: no media type" {
		mediaType = "application/octet-stream"
	} else if err != nil {
		return nil, fmt.Errorf("Media Type error: %v", err)
	}

	var parts []*Part

	if strings.HasPrefix(mediaType, "multipart/") {

		mr := multipart.NewReader(content, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, fmt.Errorf("MIME error: %v", err)
			}

			part, err := readToPart(p.Header, p)

			// XXX: maybe want to implement a less strict mode that gets what it can out of the message
			// instead of erroring out on individual sections?
			partType, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
			if err != nil {
				return nil, err
			}
			if strings.HasPrefix(partType, "multipart/") {
				subParts, err := parseContent(p.Header, bytes.NewBuffer(part.Body))
				if err != nil {
					return nil, err
				}
				part.Children = subParts
			}
			parts = append(parts, part)
		}
	} else {
		part, err := readToPart(header, content)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}

	return parts, nil
}

// Parts breaks a message body into its mime parts
func (m *Message) Parts() ([]*Part, error) {
	parts, err := parseContent(textproto.MIMEHeader(m.Header), bytes.NewBuffer(m.RawBody))
	if err != nil {
		return nil, err
	}

	return parts, nil
}

// NewMessage creates a Message from a data blob and a recipients list
func NewMessage(conn *Conn, data []byte, rcpt []*mail.Address, logger *log.Logger) (*Message, error) {
	m, err := mail.ReadMessage(bytes.NewBuffer(data))
	if err == io.EOF {
		// Empty body is allowed, but mail.ReadMessage is standard lib and throws io.EOF when it cannot
		// find a mime type section that starts the body for the message.
		// Note that this will cause message.HTML() and Header to be empty, causing errors.

		// when content-type is not included due to having no body, add it
		if !strings.Contains(string(data), "\nContent-Type:") {
			data = append(data, []byte("Content-Type: text/plain\n")...)
		}
		data = append(data, []byte("\n\n")...)
		m, err = mail.ReadMessage(bytes.NewBuffer(data))
	}
	if err != nil {
		return nil, err
	}

	// The "To": header is not required by RFC 2822, but ideally there is a CC or BCC
	to, _ := m.Header.AddressList("To")

	from, err := m.Header.AddressList("From")
	if err != nil {
		return nil, err
	}

	raw, err := ioutil.ReadAll(m.Body)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return &Message{
		Conn:    conn,
		Rcpt:    rcpt,
		To:      to,
		From:    from[0],
		Header:  m.Header,
		Subject: m.Header.Get("subject"),
		RawBody: raw,
		Source:  data,
		Logger:  logger,
	}, nil

}
