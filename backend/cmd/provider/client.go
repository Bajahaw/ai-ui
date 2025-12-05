package provider

import (
	"database/sql"
	"net/http"

	logger "github.com/charmbracelet/log"
)

var log *logger.Logger
var repo *Repo

type Client interface {
	SendChatCompletionRequest(params RequestParams) (*ChatCompletionMessage, error)
	SendChatCompletionStreamRequest(params RequestParams, w http.ResponseWriter) (*ChatCompletionMessage, error)
}

type ClientImpl struct{}

func NewClient() Client {
	return &ClientImpl{}
}

func SetupProviderClient(l *logger.Logger, db *sql.DB) {
	log = l
	repo = newProviderRepo(db)
}
