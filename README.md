# AI UI

Lightweight AI chat interface with React and Golang.

## Features

- Conversation management with history, edits, and branching
- Multi-provider AI support (OpenAI compatible ones)
- Web search integration (soon)
- Single user authentication
- Dark/light themes
- Responsive design with Shadcn UI
- Minimal resource usage (~30MB docker image)

Unlike OpenWebUI and similar heavy solutions, this is designed to be simple and fast.

## Usage

```bash
docker run -d -e APP_TOKEN={your_token} --name ai-ui ghcr.io/bajahaw/ai-ui:latest
```

Access at `http://localhost:8080`


## License
MIT
