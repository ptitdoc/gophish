package models

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/mail"
	"net/url"
	"path"
	"text/template"

	qrcode "github.com/skip2/go-qrcode"
)

// QR code image settings for the qrcode template function. Reasoning: fixed
// constants until a template actually needs another size.
const (
	qrCodeSize          = 256
	qrCodeRecoveryLevel = qrcode.Medium
)

// templateFuncs are the functions every phishing template can call, on top
// of Go's built-ins. Typical use, a QR code of the recipient URL embedded as
// a data URI: <img src="data:image/png;base64,{{.URL | qrcode | base64}}">
// (tracker:dev:2611-002).
var templateFuncs = template.FuncMap{
	"qrcode":     templateQRCode,
	"base64":     templateBase64,
	"attachment": templateAttachment, // mail bodies override it, see executeMailTemplate
}

// templateQRCode returns the PNG image of a QR code that encodes content.
func templateQRCode(content string) ([]byte, error) {
	return qrcode.Encode(content, qrCodeRecoveryLevel, qrCodeSize)
}

// templateBase64 encodes bytes (the qrcode output) or a string in standard
// base64 with padding.
func templateBase64(v interface{}) (string, error) {
	switch d := v.(type) {
	case []byte:
		return base64.StdEncoding.EncodeToString(d), nil
	case string:
		return base64.StdEncoding.EncodeToString([]byte(d)), nil
	}
	return "", fmt.Errorf("base64: unsupported type %T", v)
}

// TemplateContext is an interface that allows both campaigns and email
// requests to have a PhishingTemplateContext generated for them.
type TemplateContext interface {
	getFromAddress() string
	getBaseURL() string
}

// PhishingTemplateContext is the context that is sent to any template, such
// as the email or landing page content.
type PhishingTemplateContext struct {
	From        string
	URL         string
	Tracker     string
	TrackingURL string
	RId         string
	BaseURL     string
	BaseRecipient
}

// NewPhishingTemplateContext returns a populated PhishingTemplateContext,
// parsing the correct fields from the provided TemplateContext and recipient.
func NewPhishingTemplateContext(ctx TemplateContext, r BaseRecipient, rid string) (PhishingTemplateContext, error) {
	f, err := mail.ParseAddress(ctx.getFromAddress())
	if err != nil {
		return PhishingTemplateContext{}, err
	}
	fn := f.Name
	if fn == "" {
		fn = f.Address
	}
	templateURL, err := ExecuteTemplate(ctx.getBaseURL(), r)
	if err != nil {
		return PhishingTemplateContext{}, err
	}

	// For the base URL, we'll reset the the path and the query
	// This will create a URL in the form of http://example.com
	baseURL, err := url.Parse(templateURL)
	if err != nil {
		return PhishingTemplateContext{}, err
	}
	baseURL.Path = ""
	baseURL.RawQuery = ""

	phishURL, _ := url.Parse(templateURL)
	q := phishURL.Query()
	q.Set(RecipientParameter, rid)
	phishURL.RawQuery = q.Encode()

	trackingURL, _ := url.Parse(templateURL)
	trackingURL.Path = path.Join(trackingURL.Path, "/track")
	trackingURL.RawQuery = q.Encode()

	return PhishingTemplateContext{
		BaseRecipient: r,
		BaseURL:       baseURL.String(),
		URL:           phishURL.String(),
		TrackingURL:   trackingURL.String(),
		Tracker:       "<img alt='' style='display: none' src='" + trackingURL.String() + "'/>",
		From:          fn,
		RId:           rid,
	}, nil
}

// ExecuteTemplate creates a templated string based on the provided
// template body and data.
func ExecuteTemplate(text string, data interface{}) (string, error) {
	return executeTemplateFuncs(text, data, nil)
}

// executeTemplateFuncs renders text with templateFuncs, overridden by
// extra when it is not nil.
func executeTemplateFuncs(text string, data interface{}, extra template.FuncMap) (string, error) {
	buff := bytes.Buffer{}
	tmpl, err := template.New("template").Funcs(templateFuncs).Funcs(extra).Parse(text)
	if err != nil {
		return buff.String(), err
	}
	err = tmpl.Execute(&buff, data)
	return buff.String(), err
}

// templateAttachments collects the files a mail template adds with the
// attachment function, so the mail construction embeds them as inline parts
// (tracker:dev:2611-003). Usage, a QR code shown through its Content-ID:
// <img src="{{.URL | qrcode | attachment "invitation.png"}}">
// Reasoning: a template function cannot reach the mail being built, so the
// mail construction hands each render this collector. One collector per mail:
// its text and HTML bodies render separately and may attach the same file.
type templateAttachments struct {
	names []string // insertion order, so parts come out in template order
	files map[string][]byte
}

func newTemplateAttachments() *templateAttachments {
	return &templateAttachments{files: map[string][]byte{}}
}

// add records content under name and returns the cid: reference to it. The
// same name with the same content is recorded once; with another content it
// is an error, since one Content-ID can only show one file.
func (ta *templateAttachments) add(name string, content []byte) (string, error) {
	ref, err := templateAttachment(name, content)
	if err != nil {
		return "", err
	}
	if prev, ok := ta.files[name]; ok {
		if !bytes.Equal(prev, content) {
			return "", fmt.Errorf("attachment: %q already holds another content", name)
		}
		return ref, nil
	}
	ta.names = append(ta.names, name)
	ta.files[name] = content
	return ref, nil
}

// templateAttachment is the attachment function outside a mail (landing
// pages, subject, headers, template validation): it returns the cid:
// reference and attaches nothing, so templates using it still validate.
func templateAttachment(name string, content []byte) (string, error) {
	if name == "" {
		return "", fmt.Errorf("attachment: empty file name")
	}
	return "cid:" + name, nil
}

// executeMailTemplate renders a mail body like ExecuteTemplate, with the
// attachment function recording its files in ta.
func executeMailTemplate(text string, data interface{}, ta *templateAttachments) (string, error) {
	return executeTemplateFuncs(text, data, template.FuncMap{"attachment": ta.add})
}

// ValidationContext is used for validating templates and pages
type ValidationContext struct {
	FromAddress string
	BaseURL     string
}

func (vc ValidationContext) getFromAddress() string {
	return vc.FromAddress
}

func (vc ValidationContext) getBaseURL() string {
	return vc.BaseURL
}

// ValidateTemplate ensures that the provided text in the page or template
// uses the supported template variables correctly.
func ValidateTemplate(text string) error {
	vc := ValidationContext{
		FromAddress: "foo@bar.com",
		BaseURL:     "http://example.com",
	}
	td := Result{
		BaseRecipient: BaseRecipient{
			Email:     "foo@bar.com",
			FirstName: "Foo",
			LastName:  "Bar",
			Position:  "Test",
		},
		RId: "123456",
	}
	ptx, err := NewPhishingTemplateContext(vc, td.BaseRecipient, td.RId)
	if err != nil {
		return err
	}
	_, err = ExecuteTemplate(text, ptx)
	if err != nil {
		return err
	}
	return nil
}
