# AI UI

Lightweight AI chat interface built with React and Golang.

> [!WARNING]
> This may not be the most secure app in the world!, please use locally or in your own secure environment!

>[!NOTE]
> If you find any security issues please report. See [SECURITY.md](SECURITY.md).


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
docker run -d -p 8080:8080 --name ai-ui ghcr.io/bajahaw/ai-ui:latest
```

Access at `http://localhost:8080`

For presistant storage you need to bind `/app/data` to your file system

### ChatGPT OAuth (production)

Sign-in with ChatGPT redirects back to **your app domain**, not localhost:

`https://your-domain/api/auth/chatgpt/callback`

Set one of these on the server:

```bash
PUBLIC_BASE_URL=https://your-domain
# or the full callback URL:
# CHATGPT_OAUTH_REDIRECT_URI=https://your-domain/api/auth/chatgpt/callback
```

If unset, the backend derives the origin from the request (`Origin` / `X-Forwarded-Proto` + `Host`).


## License
MIT
