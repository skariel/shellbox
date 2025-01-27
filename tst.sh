find . -name "*.go" -not -path "./vendor/*" -exec goimports -w {} \;
go fmt ./... && golangci-lint run | grep .go | sort | uniq

