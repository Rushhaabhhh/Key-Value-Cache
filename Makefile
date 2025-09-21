build:
	go build -o key-value-cache .

run:
	./key-value-cache

docker-build:
	docker build -t key-value-cache .

docker-run:
	docker run -p 7171:7171 key-value-cache