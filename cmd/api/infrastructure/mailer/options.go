// =================================
// infrastructure/mailer/options.go
// =================================

package mailer

import "simple-arq-golang/cmd/api/infrastructure/httpclient"

type Option func(*Client)

func WithAPIKey(apiKey string) Option {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

func WithFrom(from string) Option {
	return func(c *Client) {
		c.from = from
	}
}

func WithLogger(logger httpclient.Logger) Option {
	return func(c *Client) {
		c.logger = logger
	}
}
