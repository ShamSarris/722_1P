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
ssh into azureuser@20.109.19.203:22

run ./linux-distributed.exe -primary=true -port=8080 -http=8081

This will provide the address of the primary which should be copied to the backups (10.0.0.4:8080)

# Backup 1
ssh into azureuser@40.79.241.47

run ./linux-distributed.exe -primary=false -port=8082 -http=8083

# Backup 2
(No public IP)ssh into azureuser@20.109.19.203:22
ssh into azureuser@10.0.0.6

run ./linux-distributed.exe -primary=false -port=8084 -http=8085

# Benchmark
ssh into azureuser@20.97.192.254:22

run ./linux-benchmark.exe -primary=10.0.0.4:8081 -backup1=10.0.0.4:8083 -backup2=10.0.0.5:8085

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


## One Client Testing

Default for benchmark.exe is 1 client and 10 bytes for values

### Run 1
 ./linux-benchmark.exe -primary=10.0.0.4:8081 -backup1=10.0.0.5:8083 -backup2=10.0.0.6:8085 -rwratio=0.5 -readfromlog=0 -duration=60

 Total operations: 48233
 Duration: 60.10 Seconds
 Throughput: 801.54 ops/sec

### Run 2
 ./linux-benchmark.exe -primary=10.0.0.4:8081 -backup1=10.0.0.5:8083 -backup2=10.0.0.6:8085 -rwratio=0.5 -readfromlog=0 -duration=60

 Total operations: 50842
 Duration: 60.10 Seconds
 Throughput: 845.95 ops/sec

### Run 3
 ./linux-benchmark.exe -primary=10.0.0.4:8081 -backup1=10.0.0.5:8083 -backup2=10.0.0.6:8085 -rwratio=0.5 -readfromlog=0 -duration=60

 Total operations: 49691
 Duration: 60.10 Seconds
 Throughput: 826.80 ops/sec

## Two Client Testing

### Run 1
 ./linux-benchmark.exe -primary=10.0.0.4:8081 -backup1=10.0.0.5:8083 -backup2=10.0.0.6:8085 -rwratio=0.5 -readfromlog=0 -duration=60 -clients=2

 Totel operations: 90124
 Duration: 60.10 Seconds
 Throughput: 1499.55 ops/sec

### Run 2
 ./linux-benchmark.exe -primary=10.0.0.4:8081 -backup1=10.0.0.5:8083 -backup2=10.0.0.6:8085 -rwratio=0.5 -readfromlog=0 -duration=60 -clients=2

 Total operations:70752
 Duration: 60.10 Seconds
 Throughput: 1177.23 ops/sec
 Note: Noticeable jitters in the print log for a couple seconds

### Run 3
 ./linux-benchmark.exe -primary=10.0.0.4:8081 -backup1=10.0.0.5:8083 -backup2=10.0.0.6:8085 -rwratio=0.5 -readfromlog=0 -duration=60 -clients=2

 Total operations: 60435
 Duration: 75.95 Seconds
 Throughput: 795.73 ops/sec
 Note: Significant pause near the end of duration and Request timeout error logged in primary

### Run 4
 ./linux-benchmark.exe -primary=10.0.0.4:8081 -backup1=10.0.0.5:8083 -backup2=10.0.0.6:8085 -rwratio=0.5 -readfromlog=0 -duration=60 -clients=2

 Total operations: 90691
 Duration: 60.10 Seconds
 Throughput: 1508.98 ops/sec

## Three Client testing
 ./linux-benchmark.exe -primary=10.0.0.4:8081 -backup1=10.0.0.5:8083 -backup2=10.0.0.6:8085 -rwratio=0.5 -readfromlog=0 -duration=60 -clients=3

### Run 1
 Total operations: 101357
 Duration: 88.24 Seconds
 Throughput: 1148.64 ops/sec
 Note: Pause at end of duration and request timeout logged in primary

### Run 2
 Total operations: 95982
 Duration: 89.02
 Throughput: 1078.23 ops/sec

### Run 3 NOTE: Fixed abrupt ending causing lag and low ops
 Total operations: 116822
 Duration: 65.10 Seconds
 Throughput: 1794.45 ops/sec

### Run 4
 Total operations: 103066
 Duration: 67.70 seconds
 Throughput: 1522.33 ops/sec

### Run 5: 5 Clients
 Total operations: 121939
 Duration: 81.54
 Throughput: 1495.44 ops/sec

## Multiple Clients, Read from Backups Testing
./linux-benchmark.exe -primary=10.0.0.4:8081 -backup1=10.0.0.5:8083 -backup2=10.0.0.6:8085 -rwratio=0.5 -readfromlog=1 -duration=60 -clients=2

### Run 1
Total Operations: 130167
Duration: 65.10 seconds
Throughput: 1999.46 ops/sec

### Run 2
Total operations: 132147
Duration: 65.10 seconds
Throughput: 2029.86 ops/sec

