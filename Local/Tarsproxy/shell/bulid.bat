
SET CGO_ENABLED=0
SET GOOS=linux
SET GOARCH=amd64
go build -o TarsProxy.linux

SET CGO_ENABLED=0
SET GOOS=darwin
SET GOARCH=amd64
go build -o TarsProxy.mac

SET GOOS=windows
go build -o TarsProxy.win.exe

