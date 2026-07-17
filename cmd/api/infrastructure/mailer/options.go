// =================================
// infrastructure/mailer/options.go
// =================================

package mailer

type Option func(*Client)

func WithHost(host string) Option {
	return func(c *Client) {
		c.host = host
	}
}

func WithPort(port int) Option {
	return func(c *Client) {
		c.port = port
	}
}

func WithCredentials(username, password string) Option {
	return func(c *Client) {
		c.username = username
		c.password = password
	}
}

func WithLogger(logger Logger) Option {
	return func(c *Client) {
		c.logger = logger
	}
}
