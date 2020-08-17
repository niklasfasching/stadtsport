.PHONY: run
run: build
	./stadtsport

.PHONY: update
update:
	go get -u .

.PHONY: test
test: update
	go test

.PHONY: build
build:
	go build --tags "sqlite_json"
