**raftkv is a distributed key-value store based on the raft protocol.**

Authors: Jachin Huang (hcsxiaohan@gmail.com)

# Features
* raftkv is written in Go and uses the Raft consensus algorithm to manage a highly-available replicated log.
* The basic operations are `Put(key,value)`, `Get(key)`, `Append(key,value)`.
* The fault-tolerant KV storage service is built on the basis of raft. The service can correctly handle client requests even if some nodes are wrong or the network partition.
* KV storage is managed by shards, each shard handles its own read and write.
* The distributed coordination service `shardmaster` stores the configuration information of each replica group. The configuration changes dynamically over time. The client will first communicate with it to obtain the replica group to which the key belongs. Each replica group will periodically communicate with it to get the latest service shard.
* Each replica group is responsible for processing a shard subset. As the configuration information changes, replica groups will automatically migrate data between shards to balance the load.


# Limitations
* Does not support concurrent reads and writes.
* During the shard migration process, you must wait for the migration to complete before using services.

# Raftkv architecture
![raftkv architecture](https://github.com/hchaoshun/raftkv/blob/master/raft_stream.png)

# Shardkv architecture
![shardkv architecture](https://github.com/hchaoshun/raftkv/blob/master/shardkv_stream.png)

# Testing

You can run the test code in each directory for unit testing.

For example, test raft protocol:
```bash
cd raft
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

# Next steps
TODO

# References
- [Raft Extended(2014)](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf)







