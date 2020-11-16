VERSION=0.0.6
LDFLAGS=-ldflags "-w -s -X main.Version=${VERSION}"
all: relaxlogs

.PHONY: relaxlogs

relaxlogs: relaxlogs.go logger/*.go
	go build $(LDFLAGS) -o relaxlogs

linux: relaxlogs.go logger/*.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o relaxlogs

fmt:
	go fmt ./...

clean:
	rm -rf relaxlogs

tag:
	git tag v${VERSION}
	git push origin v${VERSION}
	git push origin master
