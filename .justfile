detach := "false"

build:
  go build -o boxr cmd/boxr.go

run +command: build
  sudo ./boxr run {{ if detach == "true" { "-d" } else { "" } }}  -- {{command}}

test-pid: build
  sudo ./boxr run -r rootfs /check_pid.sh

test-pid-detached: build
  sudo ./boxr run -d -r rootfs /check_pid.sh
  sleep 1
  echo "Check container logs or use 'ps aux | grep check_pid' to verify"
