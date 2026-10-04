package msgraph

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"

	"github.com/warmbly/warmbly/internal/pkg/mailhdr"
)

// Attachment is a fully-resolved file the worker has already fetched from object
// storage, ready to be MIME-encoded.
type Attachment struct {
	Filename string
	MimeType string
	Data     []byte
}

type hdr struct{ name, value string }

// buildMIME assembles a complete RFC 5322 message from the given top-level
// headers and bodies. This is the only reliable way to control Message-ID,
// In-Reply-To/References, and custom warmup/unsubscribe headers on Graph, which
// the JSON sendMail shape cannot set. Body HTML (already carrying the tracking
// pixel + rewritten links) is preserved verbatim. Shapes emitted:
//   - text/plain only          -> single text/plain part (warmup sends)
//   - text/plain + text/html   -> multipart/alternative
//   - + attachments            -> multipart/mixed(alternative, files...)
func buildMIME(hdrs []hdr, bodyPlain, bodyHTML string, attachments []Attachment) ([]byte, error) {
	for _, h := range hdrs {
		if err := mailhdr.CheckHeader(h.name, h.value); err != nil {
			return nil, fmt.Errorf("%s: %w", h.name, err)
		}
	}
	var buf bytes.Buffer
	writeHeaders := func(extra ...hdr) {
		for _, h := range hdrs {
			fmt.Fprintf(&buf, "%s: %s\r\n", h.name, h.value)
		}
		for _, h := range extra {
			fmt.Fprintf(&buf, "%s: %s\r\n", h.name, h.value)
		}
	}

	// Plain-only: no multipart wrapper needed.
	if len(attachments) == 0 && bodyHTML == "" {
		writeHeaders(hdr{"Content-Type", "text/plain; charset=UTF-8"}, hdr{"Content-Transfer-Encoding", "quoted-printable"})
		buf.WriteString("\r\n")
		if err := writeQuotedPrintable(&buf, bodyPlain); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	// Text + HTML with no attachments: a single multipart/alternative.
	if len(attachments) == 0 {
		alt := multipart.NewWriter(&buf)
		writeHeaders(hdr{"Content-Type", fmt.Sprintf("multipart/alternative; boundary=%s", alt.Boundary())})
		buf.WriteString("\r\n")
		if err := writeAltBodies(alt, bodyPlain, bodyHTML); err != nil {
			return nil, err
		}
		return buf.Bytes(), alt.Close()
	}

	// Attachments present: multipart/mixed wrapping the alternative + one part
	// per file.
	mixed := multipart.NewWriter(&buf)
	writeHeaders(hdr{"Content-Type", fmt.Sprintf("multipart/mixed; boundary=%s", mixed.Boundary())})
	buf.WriteString("\r\n")

	var altBuf bytes.Buffer
	alt := multipart.NewWriter(&altBuf)
	if err := writeAltBodies(alt, bodyPlain, bodyHTML); err != nil {
		return nil, err
	}
	if err := alt.Close(); err != nil {
		return nil, err
	}
	altPart, err := mixed.CreatePart(textproto.MIMEHeader{
		"Content-Type": {fmt.Sprintf("multipart/alternative; boundary=%s", alt.Boundary())},
	})
	if err != nil {
		return nil, err
	}
	if _, err := altPart.Write(altBuf.Bytes()); err != nil {
		return nil, err
	}

	for _, a := range attachments {
		mimeType := a.MimeType
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		fn := mime.QEncoding.Encode("utf-8", a.Filename)
		part, perr := mixed.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {fmt.Sprintf("%s; name=%q", mimeType, fn)},
			"Content-Transfer-Encoding": {"base64"},
			"Content-Disposition":       {fmt.Sprintf("attachment; filename=%q", fn)},
		})
		if perr != nil {
			return nil, perr
		}
		if werr := writeBase64Wrapped(part, a.Data); werr != nil {
			return nil, werr
		}
	}

	return buf.Bytes(), mixed.Close()
}

// writeAltBodies writes the text/plain (and optional text/html) parts of a
// multipart/alternative body, quoted-printable encoded.
func writeAltBodies(w *multipart.Writer, bodyPlain, bodyHTML string) error {
	if err := writeTextPart(w, "text/plain; charset=UTF-8", bodyPlain); err != nil {
		return err
	}
	if bodyHTML != "" {
		if err := writeTextPart(w, "text/html; charset=UTF-8", bodyHTML); err != nil {
			return err
		}
	}
	return nil
}

func writeTextPart(w *multipart.Writer, contentType, body string) error {
	part, err := w.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {contentType},
		"Content-Transfer-Encoding": {"quoted-printable"},
	})
	if err != nil {
		return err
	}
	return writeQuotedPrintable(part, body)
}

func writeQuotedPrintable(w io.Writer, body string) error {
	qp := quotedprintable.NewWriter(w)
	if _, err := qp.Write([]byte(body)); err != nil {
		return err
	}
	return qp.Close()
}

// writeBase64Wrapped writes data as base64 hard-wrapped at 76 columns per RFC
// 2045 so strict MTAs accept the message.
func writeBase64Wrapped(w io.Writer, data []byte) error {
	encoded := base64.StdEncoding.EncodeToString(data)
	const lineLen = 76
	for i := 0; i < len(encoded); i += lineLen {
		end := i + lineLen
		if end > len(encoded) {
			end = len(encoded)
		}
		if _, err := w.Write([]byte(encoded[i:end] + "\r\n")); err != nil {
			return err
		}
	}
	return nil
}
