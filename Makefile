VERSION=0.0.3
LDFLAGS=-ldflags "-X main.Version=${VERSION}"
all: relaxlogs

.PHONY: relaxlogs

bundle:
	dep ensure

update:
	dep ensure -update

relaxlogs: relaxlogs.go
	go build $(LDFLAGS) -o relaxlogs

linux: relaxlogs.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o relaxlogs

fmt:
	go fmt ./...

clean:
	rm -rf relaxlogs

tag:
	git tag v${VERSION}
	git push origin v${VERSION}
	git push origin master
