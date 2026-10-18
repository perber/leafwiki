package backup

import (
	"net/url"
	"strings"
)

// redactedError hides credentials in a go-git transport error's message while
// keeping errors.Is and errors.As working on the original.
type redactedError struct {
	err error
	msg string
}

func (e *redactedError) Error() string { return e.msg }
func (e *redactedError) Unwrap() error { return e.err }

// redactCredentials masks the remote's credentials in err. go-git's HTTP
// transport puts the full endpoint, including a user:token part, into some
// error messages, and those messages reach logs and the status shown in the
// UI. It returns nil for nil and err itself when there is nothing to mask.
func redactCredentials(err error, cfg Config) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	redacted := msg
	var secrets []string
	if u, parseErr := url.Parse(cfg.RemoteURL); parseErr == nil && u.User != nil {
		redacted = strings.ReplaceAll(redacted, cfg.RemoteURL, u.Redacted())
		if password, ok := u.User.Password(); ok {
			secrets = append(secrets, password, url.QueryEscape(password), url.PathEscape(password))
		}
	}
	if cfg.HTTPPassword != "" {
		secrets = append(secrets, cfg.HTTPPassword)
	}
	for _, secret := range secrets {
		if secret != "" {
			redacted = strings.ReplaceAll(redacted, secret, "xxxxx")
		}
	}
	if redacted == msg {
		return err
	}
	return &redactedError{err: err, msg: redacted}
}
