BINARY  := key-value-cache
IMAGE   := key-value-cache

.PHONY: build run test race bench cover vet fmt tidy clean \
        docker-build docker-run docker-run-tuned loadtest

build: ## Compile the server binary
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BINARY) .

run: build ## Build and run the server locally
	./$(BINARY)

test: ## Run unit tests
	go test ./... -v

race: ## Run tests with the race detector
	go test -race ./...

bench: ## Run benchmarks with allocation stats
	go test -bench=. -benchmem -run=^$$ ./...

cover: ## Run tests with coverage report
	go test -cover ./...

vet: ## Static analysis
	go vet ./...

fmt: ## Format source
	gofmt -l -w .

tidy: ## Sync go.mod/go.sum
	go mod tidy

clean: ## Remove build artifacts
	rm -f $(BINARY)

docker-build: ## Build the production Docker image
	docker build -t $(IMAGE) .

docker-run: ## Run the container with sane production defaults
	docker run --rm -p 7171:7171 \
		--ulimit nofile=1048576:1048576 \
		--memory=1g --cpus=2 \
		-e GOMEMLIMIT=900MiB \
		$(IMAGE)

docker-run-tuned: ## Run with host networking + kernel tuning (see install.sh); Linux only
	./install.sh

loadtest: ## Fill a running server to its real (auto-detected) capacity and measure steady-state latency/throughput
	go run ./loadtest -fill-concurrency 300 \
		-duration 20s -concurrency 200 -read-ratio 0.9 \
		-log loadtest_results.log
