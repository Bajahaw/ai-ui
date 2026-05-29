package providers

import (
	"database/sql"

	"github.com/Bajahaw/ai-ui/cmd/utils"

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
	providers = NewRepository(db)
}
