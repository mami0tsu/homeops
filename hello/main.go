package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	env "github.com/caarlos0/env/v11"
	ssmwrap "github.com/handlename/ssmwrap/v2"
)

type RequestType int

const (
	Ping               RequestType = 1
	ApplicationCommand RequestType = 2
)

type ResponseType int

const (
	Pong    ResponseType = 1
	Message ResponseType = 4
)

type Request struct {
	Type RequestType `json:"type"`
	Data RequestData `json:"data"`
}

type RequestData struct {
	Name string `json:"name"`
}

type Response struct {
	Type ResponseType  `json:"type"`
	Data *ResponseData `json:"data,omitempty"`
}

type ResponseData struct {
	Content string `json:"content"`
}

type Config struct {
	DiscordPublicKey string `env:"DISCORD_PUBLIC_KEY,required"`
}

func NewLogger() *slog.Logger {
	opts := slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.MessageKey:
				return slog.Attr{Key: "message", Value: attr.Value}
			}
			return attr
		},
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &opts))

	return logger
}

func loadConfig(ctx context.Context) (Config, error) {
	rules := []ssmwrap.ExportRule{
		{
			Path:   "/dev/discord/public_key",
			Prefix: "DISCORD_",
		},
	}
	if err := ssmwrap.Export(ctx, rules, ssmwrap.ExportOptions{}); err != nil {
		slog.Error("failed to export parameters", slog.Any("error", err))
		return Config{}, err
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		slog.Error("failed to parse environments", slog.Any("error", err))
		return Config{}, err
	}

	return cfg, nil
}

func handleRequest(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	l := NewLogger()
	slog.SetDefault(l)

	cfg, err := loadConfig(ctx)
	if err != nil {
		slog.Error("failed to load config", slog.Any("error", err))
		return createResponse(500, "internal server error"), err
	}

	slog.Info("received request", slog.Any("request", req))

	if err := verifySignature(cfg, req); err != nil {
		slog.Error("invalid request signature", slog.Any("error", err))
		return createResponse(400, "invalid request"), err
	}

	request, err := parseRequest(req.Body)
	if err != nil {
		slog.Error("failed to parse request body", slog.Any("error", err))
		return createResponse(400, "invalid request"), err
	}

	response, err := handleRequestType(request)
	if err != nil {
		slog.Error("failed to process request", slog.Any("error", err))
		return createResponse(400, "invalid request"), err
	}

	return createResponse(200, response), nil
}

func verifySignature(cfg Config, req events.APIGatewayProxyRequest) error {
	publicKey, err := hex.DecodeString(cfg.DiscordPublicKey)
	if err != nil {
		return fmt.Errorf("public key format is invalid")
	}

	signatureHex := req.Headers["x-signature-ed25519"]
	if signatureHex == "" {
		return fmt.Errorf("signature is blank")
	}

	timestamp := req.Headers["x-signature-timestamp"]
	if timestamp == "" {
		return fmt.Errorf("timestamp is blank")
	}

	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return err
	}

	message := []byte(timestamp + req.Body)
	if !ed25519.Verify(publicKey, message, signature) {
		return fmt.Errorf("signature format is invalid")
	}

	return nil
}

func parseRequest(body string) (Request, error) {
	var request Request
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		slog.Error("failed to parse request body", slog.Any("error", err))
		return Request{}, err
	}

	return request, nil
}

func handleRequestType(req Request) (Response, error) {
	switch req.Type {
	case Ping:
		return Response{Type: Pong}, nil
	case ApplicationCommand:
		return handleCommand(req)
	default:
		return Response{}, fmt.Errorf("unknown interaction type")
	}
}

func handleCommand(req Request) (Response, error) {
	switch req.Data.Name {
	case "hello":
		return Response{
			Type: Message,
			Data: &ResponseData{
				Content: "hello, world!",
			},
		}, nil
	default:
		return Response{
			Type: Message,
			Data: &ResponseData{
				Content: "unknown command",
			},
		}, nil
	}
}

func createResponse(statusCode int, body interface{}) events.APIGatewayProxyResponse {
	respBody, err := json.Marshal(body)
	if err != nil {
		slog.Error("failed to marshal response", slog.Any("error", err))
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "failed to create response",
		}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(respBody),
	}
}

func main() {
	lambda.Start(handleRequest)
}
