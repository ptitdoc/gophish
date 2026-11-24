package models

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"

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
		URL:           fmt.Sprintf("%s?rid=%s", ctx.URL, r.RId),
		BaseURL:       ctx.URL,
		BaseRecipient: r.BaseRecipient,
		TrackingURL:   fmt.Sprintf("%s/track?rid=%s", ctx.URL, r.RId),
		From:          "From Address",
		RId:           r.RId,
	}
	expected.Tracker = "<img alt='' style='display: none' src='" + expected.TrackingURL + "'/>"
	got, err := NewPhishingTemplateContext(ctx, r.BaseRecipient, r.RId)
	c.Assert(err, check.Equals, nil)
	c.Assert(got, check.DeepEquals, expected)
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
