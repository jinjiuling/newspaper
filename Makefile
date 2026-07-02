.PHONY: build run clean docker

build:
	go mod tidy
	go build -o news .

run: build
	./news

clean:
	rm -f news
	rm -f news.db

docker:
	docker-compose build

docker-run:
	docker-compose up -d

docker-stop:
	docker-compose down
