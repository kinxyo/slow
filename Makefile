NAME := slo
MODULE := $(shell go list -m)

run:
	go build -o $(NAME) .
	@$(MAKE) install

prod:
	go build -trimpath -ldflags="-s -w" -o $(NAME) .
	@$(MAKE) install

install:
	mkdir -p ~/bin
	rm -f ~/bin/$(NAME)
	mv ./$(NAME) ~/bin/$(NAME)

test:
	go vet ./...
	go test -race -count=1 ./...

# direct imports of each package (own packages shown by name)
deps:
	@go list -f '{{.Name}}: {{join .Imports " "}}' ./... | sed 's#$(MODULE)/##g'

.PHONY: run prod install test deps
