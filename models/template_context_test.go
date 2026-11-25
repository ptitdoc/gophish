package models

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"strings"

	"github.com/gophish/gomail"
	check "gopkg.in/check.v1"
)

type mockTemplateContext struct {
	URL         string
	FromAddress string
}

func (m mockTemplateContext) getFromAddress() string {
	return m.FromAddress
}

func (m mockTemplateContext) getBaseURL() string {
	return m.URL
}

func (s *ModelsSuite) TestNewTemplateContext(c *check.C) {
	r := Result{
		BaseRecipient: BaseRecipient{
			FirstName: "Foo",
			LastName:  "Bar",
			Email:     "foo@bar.com",
		},
		RId: "1234567",
	}
	ctx := mockTemplateContext{
		URL:         "http://example.com",
		FromAddress: "From Address <from@example.com>",
	}
	expected := PhishingTemplateContext{
		URL:           fmt.Sprintf("%s?%s=%s", ctx.URL, RecipientParameter, r.RId),
		BaseURL:       ctx.URL,
		BaseRecipient: r.BaseRecipient,
		TrackingURL:   fmt.Sprintf("%s/track?%s=%s", ctx.URL, RecipientParameter, r.RId),
		From:          "From Address",
		RId:           r.RId,
	}
	expected.Tracker = "<img alt='' style='display: none' src='" + expected.TrackingURL + "'/>"
	got, err := NewPhishingTemplateContext(ctx, r.BaseRecipient, r.RId)
	c.Assert(err, check.Equals, nil)
	c.Assert(got, check.DeepEquals, expected)
}

// A recipient parameter name set in config.json ends up in the phishing and
// tracking URLs of the mail (tracker:dev:2611-001). The test sets the package
// variable the way Setup does, then puts the default back.
func (s *ModelsSuite) TestFeat_2611_001_ConfiguredRecipientParameterInURLs(c *check.C) {
	defer func() { RecipientParameter = "token" }()
	RecipientParameter = "id"
	ctx := mockTemplateContext{
		URL:         "http://example.com",
		FromAddress: "From Address <from@example.com>",
	}
	r := BaseRecipient{FirstName: "Foo", LastName: "Bar", Email: "foo@bar.com"}
	got, err := NewPhishingTemplateContext(ctx, r, "1234567")
	c.Assert(err, check.Equals, nil)
	c.Assert(got.URL, check.Equals, "http://example.com?id=1234567")
	c.Assert(got.TrackingURL, check.Equals, "http://example.com/track?id=1234567")
}

// qrTemplateContext returns a phishing context for one recipient, used by
// the QR code template function tests (tracker:dev:2611-002).
func qrTemplateContext(c *check.C, rid string) PhishingTemplateContext {
	ctx := mockTemplateContext{
		URL:         "http://example.com",
		FromAddress: "From Address <from@example.com>",
	}
	r := BaseRecipient{FirstName: "Foo", LastName: "Bar", Email: "foo@bar.com"}
	ptx, err := NewPhishingTemplateContext(ctx, r, rid)
	c.Assert(err, check.Equals, nil)
	return ptx
}

// The documented template usage renders a base64 string that decodes to a
// PNG image. The QR content itself is checked against an independent decoder
// outside this suite (see the tracker entry's acceptance criteria).
func (s *ModelsSuite) TestFeat_2611_002_QRCodeBase64RendersPNG(c *check.C) {
	ptx := qrTemplateContext(c, "1234567")
	out, err := ExecuteTemplate("{{.URL | qrcode | base64}}", ptx)
	c.Assert(err, check.Equals, nil)
	raw, err := base64.StdEncoding.DecodeString(out)
	c.Assert(err, check.Equals, nil)
	_, err = png.Decode(bytes.NewReader(raw))
	c.Assert(err, check.Equals, nil)
}

// base64 encodes a plain string with the standard alphabet and padding.
func (s *ModelsSuite) TestFeat_2611_002_Base64String(c *check.C) {
	out, err := ExecuteTemplate(`{{"abcd" | base64}}`, nil)
	c.Assert(err, check.Equals, nil)
	c.Assert(out, check.Equals, "YWJjZA==")
}

// Two recipients get two different QR codes: the image follows the URL, so
// it carries each recipient's own identifier.
func (s *ModelsSuite) TestFeat_2611_002_QRCodePerRecipient(c *check.C) {
	a, err := ExecuteTemplate("{{.URL | qrcode | base64}}", qrTemplateContext(c, "1111111"))
	c.Assert(err, check.Equals, nil)
	b, err := ExecuteTemplate("{{.URL | qrcode | base64}}", qrTemplateContext(c, "2222222"))
	c.Assert(err, check.Equals, nil)
	c.Assert(a, check.Not(check.Equals), b)
}

// An empty input cannot be encoded: the template fails instead of sending a
// mail with a broken image.
func (s *ModelsSuite) TestFeat_2611_002_QRCodeEmptyInputFails(c *check.C) {
	_, err := ExecuteTemplate(`{{"" | qrcode | base64}}`, nil)
	c.Assert(err, check.NotNil)
}

// Template validation, run when a template or page is saved, accepts both
// functions.
func (s *ModelsSuite) TestFeat_2611_002_ValidateTemplateAcceptsQRCode(c *check.C) {
	err := ValidateTemplate(`<img src="data:image/png;base64,{{.URL | qrcode | base64}}">`)
	c.Assert(err, check.Equals, nil)
}

// mailPart is one leaf part of a generated mail, its body already decoded
// from its transfer encoding.
type mailPart struct {
	header textproto.MIMEHeader
	body   []byte
}

// mailParts writes msg as it would go on the wire and returns its leaf
// parts, walking nested multiparts with a work list. Used by the attachment
// template function tests (tracker:dev:2611-003).
func mailParts(c *check.C, msg *gomail.Message) []mailPart {
	buf := &bytes.Buffer{}
	_, err := msg.WriteTo(buf)
	c.Assert(err, check.Equals, nil)
	m, err := mail.ReadMessage(buf)
	c.Assert(err, check.Equals, nil)
	body, err := ioutil.ReadAll(m.Body)
	c.Assert(err, check.Equals, nil)

	pending := []mailPart{{header: textproto.MIMEHeader(m.Header), body: body}}
	var leaves []mailPart
	for len(pending) > 0 {
		p := pending[0]
		pending = pending[1:]
		mediaType, params, err := mime.ParseMediaType(p.header.Get("Content-Type"))
		c.Assert(err, check.Equals, nil)
		if !strings.HasPrefix(mediaType, "multipart/") {
			if p.header.Get("Content-Transfer-Encoding") == "base64" {
				decoded, err := ioutil.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(p.body)))
				c.Assert(err, check.Equals, nil)
				p.body = decoded
			}
			leaves = append(leaves, p)
			continue
		}
		mr := multipart.NewReader(bytes.NewReader(p.body), params["boundary"])
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			c.Assert(err, check.Equals, nil)
			partBody, err := ioutil.ReadAll(part)
			c.Assert(err, check.Equals, nil)
			pending = append(pending, mailPart{header: part.Header, body: partBody})
		}
	}
	return leaves
}

// partsWithContentID keeps the parts a cid: reference can point to.
func partsWithContentID(parts []mailPart) []mailPart {
	var out []mailPart
	for _, p := range parts {
		if p.header.Get("Content-ID") != "" {
			out = append(out, p)
		}
	}
	return out
}

// htmlPart returns the body of the text/html part.
func htmlPart(c *check.C, parts []mailPart) string {
	for _, p := range parts {
		if strings.HasPrefix(p.header.Get("Content-Type"), "text/html") {
			return string(p.body)
		}
	}
	c.Fatal("no text/html part in the generated mail")
	return ""
}

// attachmentTestRequest returns a test mail request for one recipient whose
// template has the given text and HTML bodies.
func attachmentTestRequest(text, html string) *EmailRequest {
	return &EmailRequest{
		SMTP:     SMTP{FromAddress: "from@example.com"},
		Template: Template{Name: "QR", Subject: "Subject", Text: text, HTML: html},
		URL:      "http://example.com",
		RId:      "1234567",
		BaseRecipient: BaseRecipient{
			FirstName: "First",
			LastName:  "Last",
			Email:     "firstlast@example.com",
		},
		FromAddress: "from@example.com",
	}
}

// generateAttachmentMail runs the mail construction of a test mail and
// returns its leaf parts.
func generateAttachmentMail(c *check.C, req *EmailRequest) []mailPart {
	msg := gomail.NewMessage()
	err := req.Generate(msg)
	c.Assert(err, check.Equals, nil)
	return mailParts(c, msg)
}

// The documented usage: the HTML points to cid:<name>, and the mail carries
// one inline part with that Content-ID holding the QR code of the recipient
// URL. The QR content is checked against an independent decoder outside this
// suite (see the tracker entry's acceptance criteria).
func (s *ModelsSuite) TestFeat_2611_003_AttachmentEmbedsInlineImage(c *check.C) {
	req := attachmentTestRequest("", `<img src="{{.URL | qrcode | attachment "invitation.png"}}">`)
	ptx, err := NewPhishingTemplateContext(req, req.BaseRecipient, req.RId)
	c.Assert(err, check.Equals, nil)
	expected, err := templateQRCode(ptx.URL)
	c.Assert(err, check.Equals, nil)

	parts := generateAttachmentMail(c, req)
	c.Assert(htmlPart(c, parts), check.Equals, `<img src="cid:invitation.png">`)
	inline := partsWithContentID(parts)
	c.Assert(len(inline), check.Equals, 1)
	c.Assert(inline[0].header.Get("Content-ID"), check.Equals, "<invitation.png>")
	c.Assert(strings.HasPrefix(inline[0].header.Get("Content-Disposition"), "inline"), check.Equals, true)
	mediaType, _, err := mime.ParseMediaType(inline[0].header.Get("Content-Type"))
	c.Assert(err, check.Equals, nil)
	c.Assert(mediaType, check.Equals, "image/png")
	c.Assert(bytes.Equal(inline[0].body, expected), check.Equals, true)
}

// A mail whose template does not call attachment carries no inline part.
func (s *ModelsSuite) TestFeat_2611_003_NoAttachmentNoInlinePart(c *check.C) {
	parts := generateAttachmentMail(c, attachmentTestRequest("", `<img src="data:image/png;base64,{{.URL | qrcode | base64}}">`))
	c.Assert(len(partsWithContentID(parts)), check.Equals, 0)
}

// Two names give two inline parts, each with its own content.
func (s *ModelsSuite) TestFeat_2611_003_TwoAttachments(c *check.C) {
	parts := generateAttachmentMail(c, attachmentTestRequest("",
		`<img src="{{.URL | qrcode | attachment "a.png"}}"><img src="{{"other" | qrcode | attachment "b.png"}}">`))
	inline := partsWithContentID(parts)
	c.Assert(len(inline), check.Equals, 2)
	ids := []string{inline[0].header.Get("Content-ID"), inline[1].header.Get("Content-ID")}
	c.Assert(ids, check.DeepEquals, []string{"<a.png>", "<b.png>"})
	c.Assert(bytes.Equal(inline[0].body, inline[1].body), check.Equals, false)
}

// The text and HTML bodies are rendered separately; the same image called
// from both is attached once.
func (s *ModelsSuite) TestFeat_2611_003_SameImageInTextAndHTMLAttachedOnce(c *check.C) {
	parts := generateAttachmentMail(c, attachmentTestRequest(
		`Scan {{.URL | qrcode | attachment "invitation.png"}}`,
		`<img src="{{.URL | qrcode | attachment "invitation.png"}}">`))
	c.Assert(len(partsWithContentID(parts)), check.Equals, 1)
}

// Two recipients get two different images under the same name.
func (s *ModelsSuite) TestFeat_2611_003_AttachmentPerRecipient(c *check.C) {
	html := `<img src="{{.URL | qrcode | attachment "invitation.png"}}">`
	reqA := attachmentTestRequest("", html)
	reqB := attachmentTestRequest("", html)
	reqB.RId = "7654321"
	a := partsWithContentID(generateAttachmentMail(c, reqA))
	b := partsWithContentID(generateAttachmentMail(c, reqB))
	c.Assert(len(a), check.Equals, 1)
	c.Assert(len(b), check.Equals, 1)
	c.Assert(bytes.Equal(a[0].body, b[0].body), check.Equals, false)
}

// One name cannot hold two different contents: the second call fails
// instead of silently dropping an image.
func (s *ModelsSuite) TestFeat_2611_003_SameNameDifferentContentFails(c *check.C) {
	tmpl := `{{.URL | qrcode | attachment "x.png"}}{{"other" | qrcode | attachment "x.png"}}`
	_, err := executeMailTemplate(tmpl, qrTemplateContext(c, "1234567"), newTemplateAttachments())
	c.Assert(err, check.ErrorMatches, `.*attachment: "x.png" already holds another content.*`)
}

// An empty file name fails: a part with no name cannot be referenced.
func (s *ModelsSuite) TestFeat_2611_003_EmptyNameFails(c *check.C) {
	_, err := ExecuteTemplate(`{{.URL | qrcode | attachment ""}}`, qrTemplateContext(c, "1234567"))
	c.Assert(err, check.ErrorMatches, `.*attachment: empty file name.*`)
	_, err = executeMailTemplate(`{{.URL | qrcode | attachment ""}}`, qrTemplateContext(c, "1234567"), newTemplateAttachments())
	c.Assert(err, check.ErrorMatches, `.*attachment: empty file name.*`)
}

// Only bytes can be attached; a plain string is a template error.
func (s *ModelsSuite) TestFeat_2611_003_StringContentFails(c *check.C) {
	_, err := executeMailTemplate(`{{"text" | attachment "x.png"}}`, qrTemplateContext(c, "1234567"), newTemplateAttachments())
	c.Assert(err, check.ErrorMatches, `.*wrong type for value; expected \[\]uint8; got string.*`)
}

// Outside a mail (landing page, subject, headers, template validation) the
// function only returns the cid: reference and attaches nothing.
func (s *ModelsSuite) TestFeat_2611_003_OutsideMailReturnsCIDOnly(c *check.C) {
	out, err := ExecuteTemplate(`{{.URL | qrcode | attachment "invitation.png"}}`, qrTemplateContext(c, "1234567"))
	c.Assert(err, check.Equals, nil)
	c.Assert(out, check.Equals, "cid:invitation.png")
}

// Template validation, run when a template is saved, accepts the function.
func (s *ModelsSuite) TestFeat_2611_003_ValidateTemplateAcceptsAttachment(c *check.C) {
	err := ValidateTemplate(`<img src="{{.URL | qrcode | attachment "invitation.png"}}">`)
	c.Assert(err, check.Equals, nil)
}
