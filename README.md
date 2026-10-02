

# AI UI

Lightweight AI chat interface built with React and Golang.

> [!WARNING]
> This may not be the most secure app in the world!, please use locally or in your own secure environment!

>[!NOTE]
> If you find any security issues please report. See [SECURITY](SECURITY.md).


<img width="2879" height="1482" alt="image (1)" src="https://github.com/user-attachments/assets/c1b0fa60-8144-43ea-a7be-5b2c358ae9da" />


## Features

- Conversations, history, edits, and branching
- Multi-provider AI support (OpenAI compatible ones)
- Sign in with ChatGPT — connect your ChatGPT account as a provider
- Multi-user support.
- Tools/MCP integration
- Agentic document retrieval.
- Advanced chat features.
- Dark/light themes
- Blazing fast UI.
- Minimal resource usage (~30MB)

Unlike OpenWebUI and similar heavy solutions, this is designed to be simple and fast.

## Usage

```bash
# Generate once and store it somewhere safe (e.g. a password manager)
export ENCRYPTION_KEY="$(openssl rand -base64 32)"

docker run -d -p 8080:8080 -e ENCRYPTION_KEY --name ai-ui ghcr.io/bajahaw/ai-ui:latest
```

Access at `http://localhost:8080`

For persistent storage you need to bind `/app/data` to your file system

### Configuration

- `ENCRYPTION_KEY` is required! If it is lost, stored API keys and tokens cannot be recovered.

See [.env.example](.env.example) for all options.

## License
MIT
