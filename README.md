**raftkv is a distributed key-value store based on the raft protocol.**

raftkv is written in Go and uses the [Raft][raft] consensus algorithm to manage a highly-available replicated log.

Authors: Jachin Huang (hcsxiaohan@gmail.com)

# Features
* raftkv is written in Go and uses the [Raft][raft] consensus algorithm to manage a highly-available replicated log.
* The basic operations are `Put(key,value)`, `Get(key)`, `Append(key,value)`.
* The fault-tolerant KV storage service is built on the basis of raft. The service can correctly handle client requests even if some nodes are wrong or the network partition.
* KV storage is managed by shards, each shard handles its own read and write.
* The distributed coordination service shardmaster stores the configuration information of each replica group. The configuration changes dynamically over time. The client will first communicate with it to obtain the replica group to which the key belongs. Each replica group will periodically communicate with it to get the latest service shard.
* Each replica group is responsible for processing a shard subset. As the configuration information changes, replica groups will automatically migrate data between shards to balance the load.


# Limitations
* Does not support concurrent reads and writes.
* During the shard migration process, you must wait for the migration to complete before using services.


# Testing

test raft protocol:
```bash
cd raft
go test -run ''
```

test kv service:
```bash
cd kvraft
go test -run ''
```

test shardmaster service:
```bash
cd shardmaster
go test -run ''
```

test shard kv service:
```bash
cd shardkv
go test -run ''
```

# Performance

Here is a performance test report.

## Setup
TODO

## Write performance
TODO

## Read performance
TODO

# Repository contents

Guide to per files:

TODO







