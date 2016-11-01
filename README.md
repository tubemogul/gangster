## Synopsis

Gangster is a small daemon written in Golang that polls gmond for the metrics and sends them to the Carbon using plain-text protocol.

## Code Example

Gangster reads its configuration parameters from the environment variables. To see all the available parameters just start it with the `help` argument.

```
./gangster help
GANGSTER_GMOND_ADDRESS [mandatory]: address which gmond listens. Exmaple: 127.0.0.1
GANGSTER_GMOND_PORT [mandatory]: port which listens gmond. Example: 8649
GANGSTER_CARBON_ADDRESS [mandatory]: address where gangster should send metrics. Example: carbon01
GANGSTER_CARBON_PORT [mandatory]: port where gangster should send metrics. Example: 2003
GANGSTER_CARBON_PROTOCOL [mandatory]: protocol which gangster should use for sending. Example: udp
GANGSTER_GRAPHITE_PREFIX: prefix for metrics. Example: zone.mgmt
GANGSTER_LOG_FILE: log file location, Example: /mnt/gangster.log
GANGSTER_CLUSTER_AS_A_PREFIX: use Ganglia cluster as a prefix for graphite metric
GANGSTER_SLEEP_TIME: sleep time between gmond polls
```

## Installation

It has no non-standart requirements, so you can just compile with the `go build` command.
