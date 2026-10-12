include local.env
include .secrets
export

test:
	@echo "Running tests"
	go run github.com/onsi/ginkgo/v2/ginkgo -r -p --keep-going --randomize-all

race:
	@echo "Running tests with the race detector"
	go run github.com/onsi/ginkgo/v2/ginkgo -r -p --keep-going --randomize-all --race

expensive:
	@echo "Running Expensive tests"
	INCLUDE_LLM_SPECS=true go run github.com/onsi/ginkgo/v2/ginkgo -r -p --keep-going --randomize-all

local:
	@echo "Running disco"
	@go run main.go

ensure-compiles:
	@go build .
	@rm disco

deploy: test ensure-compiles
	@echo "Deploying 🪩"
	@flyctl deploy

shipit: deploy