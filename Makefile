all: search-console

search-console: search-console.go
	@echo Compiling $@...
	@go build -o $@ $<
