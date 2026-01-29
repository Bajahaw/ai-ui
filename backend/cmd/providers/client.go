package providers

import (
	"ai-client/cmd/utils"
	"database/sql"

	logger "github.com/charmbracelet/log"
)

var log *logger.Logger
var providers Repository

type Client interface {
	SendChatCompletionRequest(params RequestParams) (*ChatCompletionMessage, error)
	SendChatCompletionStreamRequest(params RequestParams, sc utils.StreamClient) (*ChatCompletionMessage, error)
}

type ClientImpl struct{}

func NewClient() Client {
	return &ClientImpl{}
}

func SetupProviderClient(l *logger.Logger, db *sql.DB) {
	log = l
	providers = newProviderRepo(db)
}
