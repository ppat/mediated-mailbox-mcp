package consent

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// ParseRedirect reads the address the browser was sent to after the consent, as the operator pasted
// it. It returns mail.ErrWrongAddress unless the address is redirectURI's, the scheme, host, port and
// path alike. An address pasted without its scheme is read as plain HTTP, since a browser's address
// bar may leave it out. Surrounding whitespace is ignored. The state, the code and the error it
// carries are returned as read.
func ParseRedirect(address, redirectURI string) (mail.Redirect, error) {
	want, err := url.Parse(redirectURI)
	if err != nil {
		return mail.Redirect{}, err
	}
	text := strings.TrimSpace(address)
	if !strings.Contains(text, "://") {
		text = "http://" + text
	}
	got, err := url.Parse(text)
	if err != nil || got.Scheme != want.Scheme || got.Host != want.Host || got.User != nil || !samePath(got.Path, want.Path) {
		return mail.Redirect{}, fmt.Errorf("gmail: the pasted address is not %s: %w", redirectURI, mail.ErrWrongAddress)
	}
	q := got.Query()
	return mail.Redirect{State: q.Get("state"), Code: q.Get("code"), Error: q.Get("error")}, nil
}

// samePath treats an empty path as the root, which a browser shows either way.
func samePath(got, want string) bool {
	if got == "" {
		got = "/"
	}
	return got == want
}
