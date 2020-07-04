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

.PHONY: deploy
deploy: build
	rsync -rv --copy-links --delete stadtsport assets config.json bmo@niklasfasching.de:~/stadtsport
	ssh bmo@niklasfasching.de 'sleep 1; systemctl --user restart stadtsport; sleep 1; systemctl --user status stadtsport'

.PHONY: logs
logs:
	ssh bmo@niklasfasching.de journalctl --user-unit stadtsport -f -n300
