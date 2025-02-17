detach := "false"

build: 
  go build -o boxr cmd/main.go

run +command: build
  sudo ./boxr run {{ if detach == "true" { "-d" } else { "" } }}  -- {{command}}
