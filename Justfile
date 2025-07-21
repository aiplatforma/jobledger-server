build-css:
  cd internal/templates && pnpm run css

run: build-css
  go run cmd/server/main.go
