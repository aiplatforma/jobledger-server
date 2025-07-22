set dotenv-filename := ".env.dev"

build-css:
  cd internal/templates && pnpm run css

run: build-css
  go run cmd/server/main.go
