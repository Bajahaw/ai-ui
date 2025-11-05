package tools

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"iter"
	"net/http"
	"time"

	logger "github.com/charmbracelet/log"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var log *logger.Logger
var db *sql.DB
var toolCallsRepo ToolCallsRepository

func SetUpTools(l *logger.Logger, database *sql.DB) {
	db = database
	toolCallsRepo = NewToolCallsRepository(db)
	log = l
}

func ListMCPFeatures() {
	// Use context.Background() for the session (SSE needs long-lived connection)
	// For individual operations, you can use context.WithTimeout separately
	ctx := context.Background()

	// Create a new client, with no features.
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)

	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		// Endpoint: "https://mcp.supabase.com/mcp?project_ref=yxljexzkzveojrlxjvya",
		// Endpoint: "https://mcp.tavily.com/mcp",
		Endpoint: "https://mcp.firecrawl.dev/mcp",

		HTTPClient: customHTTPClient,
	}, nil)

	if err != nil {
		log.Fatalf("Error connecting to MCP server: %v", err)
		return
	}
	defer session.Close()

	// Create a context with timeout for listing operations only
	listCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if session.InitializeResult().Capabilities.Tools != nil {
		printSection("tools", session.Tools(listCtx, nil), func(t *mcp.Tool) string { return t.Name })
	}
	if session.InitializeResult().Capabilities.Resources != nil {
		printSection("resources", session.Resources(listCtx, nil), func(r *mcp.Resource) string { return r.Name })
		printSection("resource templates", session.ResourceTemplates(listCtx, nil), func(r *mcp.ResourceTemplate) string { return r.Name })
	}
	if session.InitializeResult().Capabilities.Prompts != nil {
		printSection("prompts", session.Prompts(listCtx, nil), func(p *mcp.Prompt) string { return p.Name })
	}

}

func printSection[T any](name string, features iter.Seq2[T, error], featName func(T) string) {
	fmt.Printf("%s:\n", name)
	for feat, err := range features {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\t%s\n", featName(feat))
	}
	fmt.Println()
}

type acceptHeaderRoundTripper struct {
	delegate http.RoundTripper
}

func (rt *acceptHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {

	// req.Header.Set("Authorization", "Bearer fc-***")
	// req.Header.Set("Cache-Control", "no-cache")
	// req.Header.Set("Content-Type", "application/json")
	// req.Header.Set("Accept", "application/json, text/event-stream")

	// Debug logging - uncomment if needed for troubleshooting
	log.Debug("request url", "url", req.URL)
	log.Debug("request headers", "headers", req.Header)
	log.Debug("request method", "method", req.Method)
	// log.Debug("request params", "params", req.URL.Query())

	// Read and restore the request body (required to avoid consuming the body stream)
	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			log.Debug("error reading request body", "error", err)
			return nil, err
		}
		log.Debug("request body", "body", string(bodyBytes))
		// Restore the body so it can be read again
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	resp, err := rt.delegate.RoundTrip(req)
	if err != nil {
		// log.Errorf("[DEBUG] HTTP Request Error: %v", err)
		return nil, err
	}

	log.Debug("response headers", "headers", resp.Header)
	log.Debug("response status", "status", resp.Status)

	// Only log response body for non-streaming responses
	// SSE responses (text/event-stream) should not be read here as they're long-lived streams
	// contentType := resp.Header.Get("Content-Type")
	// if resp.Body != nil {
	// 	// if resp.Body != nil && contentType != "text/event-stream" {
	// 	bodyBytes, err := io.ReadAll(resp.Body)
	// 	if err != nil {
	// 		log.Error("error reading response body", "error", err)
	// 		return nil, err
	// 	}
	// 	log.Debug("response body", "body", string(bodyBytes))
	// 	// Restore the body so it can be read again
	// 	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	// } else if contentType == "text/event-stream" {
	// 	log.Debug("response body", "body", "(SSE stream - not logged)")
	// }

	return resp, err
}

var customHTTPClient = &http.Client{
	Transport: &acceptHeaderRoundTripper{
		delegate: http.DefaultTransport,
	},
}
