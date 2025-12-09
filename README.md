# AI UI

Lightweight AI chat interface built with React and Golang.

> [!WARNING]
> This is not intended for production use, please use locally or in your own secure environment!

## Features

- Conversation management with history, edits, and branching
- Multi-provider AI support (OpenAI compatible ones)
- Tools/MCP integration
- Built-in Backups (planned) 
- Single user authentication
- Dark/light themes
- Responsive design with Shadcn UI
- Minimal resource usage (~30MB docker image)

Unlike OpenWebUI and similar heavy solutions, this is designed to be simple and fast.

## Usage

```bash
docker run -d -p 8080:8080 --name ai-ui ghcr.io/bajahaw/ai-ui:latest
```

Access at `http://localhost:8080`

For presistant storage you need to bind `/app/data` to your file system


## License
MIT
