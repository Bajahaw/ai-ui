# AI UI - Frontend

A modern AI chat interface built with React, TypeScript, Vite, and shadcn/ui.

## Features

- 🚀 Fast development with Vite
- ⚛️ React 19 with TypeScript
- 🎨 shadcn/ui + Tailwind CSS
- 🤖 AI chat interface
- 📱 Responsive design
- 🌙 Dark/Light theme toggle
- 🔧 ESLint configuration
- 🎯 Component library with AI-specific elements

## Prerequisites

- Node.js 18+ 
- npm or yarn

## Getting Started

### 1. Install Dependencies

```bash
npm install
```

### 2. Development

```bash
npm run dev
```

The app will be available at `http://localhost:3000`. 

The frontend expects a Go backend API running on `http://localhost:8080`.

### 3. Build for Production

```bash
npm run build
```

The built files will be in the `dist` directory.

### 4. Preview Production Build

```bash
npm run preview
```

## Project Structure

```
ai-ui/
├── src/
│   ├── components/          # Reusable UI components
│   │   ├── ai-elements/     # AI-specific components
│   │   └── ui/             # shadcn/ui components
│   ├── lib/                # Utilities and helpers
│   ├── App.tsx             # Main application component
│   ├── main.tsx            # Application entry point
│   └── globals.css         # Global styles with theme variables
├── public/                 # Static assets
├── index.html              # HTML entry point
├── vite.config.ts          # Vite configuration
├── tailwind.config.js      # Tailwind CSS configuration
├── components.json         # shadcn/ui configuration
└── tsconfig.json           # TypeScript configuration
```

## Available Scripts

- `npm run dev` - Start development server
- `npm run build` - Build for production
- `npm run preview` - Preview production build
- `npm run lint` - Run ESLint

## Backend API

The frontend is configured to proxy API calls to `http://localhost:8080` during development. 

### API Endpoint

The app expects a Go backend with the following endpoint:

**POST /api/chat**

**Request:**
```json
{
  "messages": [
    {
      "role": "user",
      "content": "Hello, how are you?"
    }
  ],
  "model": "openai/gpt-4o",
  "webSearch": false
}
```

**Response:**
```json
{
  "message": {
    "id": "msg_123",
    "role": "assistant",
    "content": "I'm doing well, thank you!"
  }
}
```

## Theme Support

The app includes a complete dark/light theme implementation using:

- **Theme Provider**: React context for theme management
- **Theme Toggle**: Simple button that toggles between light and dark modes
- **System Aware**: Automatically follows system preference by default
- **CSS Variables**: Automatic theme switching
- **shadcn/ui Compatible**: Follows shadcn/ui theming conventions

Theme preference is persisted in localStorage and automatically respects system preferences when first loaded.

## Component Library

The app includes shadcn/ui components and AI-specific components:

### UI Components (shadcn/ui)
- Button, Input, Select, Textarea
- Dropdown Menu, Tooltip, Avatar
- Badge, Scroll Area
- Theme Toggle

### AI Elements
- **Conversation**: Chat container, content area, scroll controls
- **Message**: Message bubbles, content formatting
- **Prompt Input**: Text input, model selection, toolbar, submit controls
- **Response**: Markdown rendering with syntax highlighting
- **Loader**: Loading states

## Styling

- **Tailwind CSS**: Utility-first CSS framework
- **CSS Variables**: Theme customization with automatic light/dark mode
- **shadcn/ui**: Consistent component styling
- **Component Variants**: Built with class-variance-authority

## Development Notes

- React 19 with modern hooks and concurrent features
- TypeScript strict mode enabled
- ESLint configured for React and TypeScript
- Vite for fast development and optimized builds
- All backend/mock code removed - frontend only

## Deployment

### Static Hosting
Deploy the `dist` folder to any static hosting service:
- Vercel, Netlify, GitHub Pages
- AWS S3 + CloudFront
- Any CDN or web server

### Backend Requirements
Ensure your Go backend:
1. Runs on `localhost:8080` (or update proxy in `vite.config.ts`)
2. Implements the `/api/chat` endpoint
3. Handles CORS for your frontend domain
4. Returns responses in the expected format

### Production Configuration
For production, configure your web server to:
- Serve static files from `dist/`
- Proxy `/api/*` requests to your Go backend
- Handle client-side routing (return `index.html` for SPA routes)

## Future Enhancements

- [ ] Add authentication UI
- [ ] Implement conversation history
- [ ] Add file upload support
- [ ] Enhanced streaming responses
- [ ] Better error handling
- [ ] PWA support