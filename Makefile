build:
	rm -rf ./be.pb.go go.mod go.sum
	go mod init
	go build -ldflags="-w -s"

