package email

import (
	"bytes"
	htmltemplate "html/template"
	texttemplate "text/template"
)

// templateData is deliberately minimal — v1 templates carry only a link, no
// username or site name, so there is no user-controlled interpolation to
// worry about injecting through (html/template still auto-escapes on top of
// that as a second layer, but the real defense is not accepting that input
// at all).
type templateData struct {
	Link string
}

var (
	passwordResetHTMLTmpl = htmltemplate.Must(htmltemplate.New("password_reset_html").Parse(passwordResetHTMLSource))
	passwordResetTextTmpl = texttemplate.Must(texttemplate.New("password_reset_text").Parse(passwordResetTextSource))
	inviteHTMLTmpl        = htmltemplate.Must(htmltemplate.New("invite_html").Parse(inviteHTMLSource))
	inviteTextTmpl        = texttemplate.Must(texttemplate.New("invite_text").Parse(inviteTextSource))
)

const passwordResetHTMLSource = `<p>Hello,</p>
<p>We received a request to reset your LeafWiki password. Click the link below to choose a new one:</p>
<p><a href="{{.Link}}">{{.Link}}</a></p>
<p>This link expires in 1 hour. If you didn't request this, you can safely ignore this email.</p>`

const passwordResetTextSource = `Hello,

We received a request to reset your LeafWiki password. Open the link below to choose a new one:

{{.Link}}

This link expires in 1 hour. If you didn't request this, you can safely ignore this email.
`

const inviteHTMLSource = `<p>Hello,</p>
<p>You've been invited to a LeafWiki instance. Click the link below to set your password and get started:</p>
<p><a href="{{.Link}}">{{.Link}}</a></p>
<p>This link expires in 7 days.</p>`

const inviteTextSource = `Hello,

You've been invited to a LeafWiki instance. Open the link below to set your password and get started:

{{.Link}}

This link expires in 7 days.
`

func renderPasswordReset(link string) (htmlBody, textBody string, err error) {
	return renderPair(passwordResetHTMLTmpl, passwordResetTextTmpl, link)
}

func renderInvite(link string) (htmlBody, textBody string, err error) {
	return renderPair(inviteHTMLTmpl, inviteTextTmpl, link)
}

func renderPair(htmlTmpl *htmltemplate.Template, textTmpl *texttemplate.Template, link string) (string, string, error) {
	data := templateData{Link: link}

	var htmlBuf bytes.Buffer
	if err := htmlTmpl.Execute(&htmlBuf, data); err != nil {
		return "", "", err
	}

	var textBuf bytes.Buffer
	if err := textTmpl.Execute(&textBuf, data); err != nil {
		return "", "", err
	}

	return htmlBuf.String(), textBuf.String(), nil
}
