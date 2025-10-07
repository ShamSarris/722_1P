# Kyle + Sam CS722 1P

## Run Instructions
To build for windows:

$ go build 
To build for linux (in powershell with admin priv):

$ $env:GOOS="linux"
$ go build -o distributed-linux
(Remember to set you GOOS back to "windows" to build for windows)

In seperate terminals/VMs run (use distributed.exe for windows)(Can be any ports):

# Primary
./distributed -primary=true -port=8080 -http=8081

# Backup 1
./distributed -primary=false -port=8082 -http=8083

# Backup 2
./distributed -primary=false -port=8084 -http=8085

## Features and Implementation

###LSN Ordering

    - Both READ and WRITE operations consume LSN slots
    - Backups track lastAppliedLSN to ensure sequential application
    - Out-of-order commits are queued until previous LSNs are applied
    - This prevents inconsistencies when messages arrive out of order

## AI Usage

Anthropic Claude (Sonnet 3.5-4.5) was mainly used as the driving agent for code refactoring from project 0P. We needed to update our code to run actors
on HTTP servers and accept API style requests from other clients. The HTTP server code was relatively trivial, however implementating storage and 
LSN ordering was far more complex. We used Claude to guide us through implementing this behavior, including prompts like "Help me refactor
my original code to accept curl requests from the command line and correctly order and commit the requests."

Claude was also used to help create the test driver, which is instrumental to our success in this project. It made both starting the servers, as well as
testing that they were still functioning as we expected as we continue to develop. 



