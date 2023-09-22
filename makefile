include local.env
include .secrets
export

test:
	@echo "Running tests"
	ginkgo -r -p --keep-going --randomize-all

local:
	@echo "Running disco"
	@go run main.go

deploy: test
	@echo "Deploying 🪩"
	@flyctl deploy