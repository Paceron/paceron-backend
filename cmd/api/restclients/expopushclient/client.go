package expopushclient

import (
	"context"

	"simple-arq-golang/cmd/api/infrastructure/httpclient"
)

const sendPath = "/--/api/v2/push/send"

// ExpoPushClientInterface envía notificaciones push a través del servicio público de
// Expo — sin SDK, HTTP plano contra https://exp.host, mismo patrón que cualquier otro
// restclient de este repo (ver exampleweatherclient).
type ExpoPushClientInterface interface {
	Send(ctx context.Context, token, title, body string, data map[string]string) error
}

type expoPushClient struct {
	httpClient *httpclient.Client
}

func New(httpClient *httpclient.Client) ExpoPushClientInterface {
	return &expoPushClient{
		httpClient: httpClient,
	}
}

type sendPushRequest struct {
	To    string            `json:"to"`
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Data  map[string]string `json:"data,omitempty"`
}

func (c *expoPushClient) Send(ctx context.Context, token, title, body string, data map[string]string) error {
	req := sendPushRequest{
		To:    token,
		Title: title,
		Body:  body,
		Data:  data,
	}

	return c.httpClient.Post(ctx, sendPath, req, nil)
}
