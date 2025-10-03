# Kyle + Sam CS722 0P


## Run Instructions
To build for windows:

$ go build 
To build for linux (in powershell with admin priv):

$ $env:GOOS="linux"
$ go build -o distributed-linux
(Remember to set you GOOS back to "windows" to build for windows)

In seperate terminals/VMs run (use distributed.exe for windows)(Can be any ports):


# Primary
./distributed -primary=true -host=127.0.0.1 -port=8080 -http=8081

# Backup 1
./distributed -primary=false -host=127.0.0.1 -port=8082 -http=8083

# Backup 2
./distributed -primary=false -host=127.0.0.1 -port=8084 -http=8085