# MoodMarket

Mood-driven daily investment advisor powered by Claude.

## Quick start

### 1. Add your API key
Edit backend/.env and replace the placeholder with your Anthropic API key.

### 2. Start the backend
  cd backend
  go run cmd/server/main.go

### 3. Test it (new terminal tab)
  curl -X POST http://localhost:8080/recommend \
    -H "Content-Type: application/json" \
    -d '{"mood":"warm","base_budget":100,"extra_money":0}'

### 4. Start the frontend (new terminal tab)
  cd frontend
  npm create vite@latest . -- --template react-ts
  npm install
  npm run dev

Then open http://localhost:5173
