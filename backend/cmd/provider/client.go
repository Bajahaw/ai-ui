package provider

import (
	"database/sql"
	"net/http"

	logger "github.com/charmbracelet/log"
	"github.com/openai/openai-go/v3"
)

var log *logger.Logger
var repo *Repo

type ProviderClient interface {
	SendChatCompletionRequest(params ProviderRequestParams) (*openai.ChatCompletion, error)
	SendChatCompletionStreamRequest(params ProviderRequestParams, w http.ResponseWriter) (*openai.ChatCompletionMessage, error)
}

type Client struct{}

func SetupProviderClient(l *logger.Logger, db *sql.DB) {
	log = l
	repo = newProviderRepo(db)
}
