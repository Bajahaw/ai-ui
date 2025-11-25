package provider

import (
	"database/sql"
	"net/http"

	logger "github.com/charmbracelet/log"
	"github.com/openai/openai-go/v3"
)

var log *logger.Logger
var repo *Repo

type Client interface {
	SendChatCompletionRequest(params RequestParams) (*openai.ChatCompletion, error)
	SendChatCompletionStreamRequest(params RequestParams, w http.ResponseWriter) (*openai.ChatCompletionMessage, error)
}

type ClientImpl struct{}

func SetupProviderClient(l *logger.Logger, db *sql.DB) {
	log = l
	repo = newProviderRepo(db)
}
