# go-thai-smartcard

Go application read personal and nhso data from thai id card, it run in the background and wait until inserted card then send readed data to everyone via [https://socket.io/](socket.io) and [WebSockets](https://developer.mozilla.org/en-US/docs/Web/API/WebSockets_API).

## How to build test

- Required version [Go](https://go.dev/dl/) version 1.18+
- Clone this repository
- Download all depencies with `go mod download`

> Linux install `sudo apt install build-essential libpcsclite-dev pcscd`

- Build with `go build -o bin/thai-smartcard-agent ./main.go`

  > Windows `go build -o bin/thai-smartcard-agent.exe ./main.go`

## How to GUI

- install fyne : 'go install fyne.io/fyne/v2/cmd/fyne@latest'
- For MacOS : 'fyne package -os darwin -icon icon.png'
- for windows : 'go build -ldflags "-H windowsgui" -o ./bin/thai-smartcard-gui main.go'

## How to change icon in systemtray
- using command "fyne bundle icon.png > bundled.go"


