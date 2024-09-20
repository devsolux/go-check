# Go Check

Check for outdated go module.

## Getting Started

```sh
go mod tidy

go run github.com/devsolux/go-check@latest
```

## Use Case
```bash
go install github.com/devsolux/go-check@latest
go version
go mod init go-check
go mod tidy

go run main.go
```

## Clean
```bash
go clean -cache -modcache -testcache

# Clear build cache
go clean -cache

# Clear module cache
go clean -modcache

# Clear test cache
go clean -testcache
```