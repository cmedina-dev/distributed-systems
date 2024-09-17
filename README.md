# Distributed Systems

The goal is to use [Maelstrom's distributed systems workbench](https://github.com/jepsen-io/maelstrom) to complete a series of challenges titled "Gossip Glomers."

Gossip Glomers increases in difficulty with each challenge. The challenges are:
1. Echo
2. Unique ID Generation
3. Broadcast
4. Grow-Only Counter
5. Kafka-Style Log
6. Totally-Available Transactions

For each challenge after Echo, the results, latency quantiles, raw latency, throughput, and timeline can be found in the respective challenge's folder.

### Echo

This challenge serves as an introduction to Maelstrom and its capability of testing your code in a distributed environment under load.

### Unique ID Generation

The first actual challenge of Gossip Glomers requires that nodes list for requests and respond with a unique ID to each request.
IDs must be unique across the cluster.

My solution to this challenge involved using a linearized key-value store which incremented a counter with a Compare-and-Swap approach, which the nodes would use as the unique ID.

```text
{:perf {:latency-graph {:valid? true},
        :rate-graph {:valid? true},
        :valid? true},
 :timeline {:valid? true},
 :exceptions {:valid? true},
 :stats {:valid? true,
         :count 27311,
         :ok-count 27311,
         :fail-count 0,
         :info-count 0,
         :by-f {:generate {:valid? true,
                           :count 27311,
                           :ok-count 27311,
                           :fail-count 0,
                           :info-count 0}}},
 :availability {:valid? true, :ok-fraction 1.0},
 :net {:all {:send-count 231982,
             :recv-count 231982,
             :msg-count 231982,
             :msgs-per-op 8.494086},
       :clients {:send-count 54628,
                 :recv-count 54628,
                 :msg-count 54628},
       :servers {:send-count 177354,
                 :recv-count 177354,
                 :msg-count 177354,
                 :msgs-per-op 6.493867},
       :valid? true},
 :workload {:valid? true,
            :attempted-count 27311,
            :acknowledged-count 27311,
            :duplicated-count 0,
            :duplicated {},
            :range [0 27310]},
 :valid? true}
```

### (Efficient) Broadcast

The next challenge was broken into five stages. Each stage could be summarized as:

1. Single-Node Broadcast - Receiving a message and storing it within a node
2. Multi-Node Broadcast - Taking that message and disseminating it among peers
3. Fault Tolerant Broadcast - Ensuring that message propagates regardless of network partitions
4. Efficient Broadcast, Part I - Optimizing messages from the original naive broadcast protocol
5. Efficient Broadcast, Part II - Further optimizing the protocol under stricter network conditions

This challenge represented a significant step-up in difficulty as it required the implementation of several
distributed systems patterns. Namely, in my approach, I utilized the following:
* Vector clocks
* Deltas
* Gossip protocol
* Heartbeats
* Atomic operations
* Message batching

All of the above were new to me as I approached this challenge, so I needed to devote a significant amount of time
to researching each of these distributed systems patterns. Some of them came naturally, such as the gossip protocol,
heartbeats, and message batching, while others such as vector clocks and deltas took more work.

The final result was an optimized broadcast protocol which efficiently handles large amounts of message transactions
with very few operations. Moreover, by lowering the tick rate of the system, the number of operations can be decreased
even further at the cost of slightly higher latency. Ultimately I settled at 400ms for synchronization which adhered
to the challenge's requirements:

> With the same node count of 25 and a message delay of 100ms, your challenge is to achieve the following performance metrics:
> 
> * Messages-per-operation is below 20
> * Median latency is below 1 second
> * Maximum latency is below 2 seconds 

```text
{:perf {:latency-graph {:valid? true},
        :rate-graph {:valid? true},
        :valid? true},
 :timeline {:valid? true},
 :exceptions {:valid? true},
 :stats {:valid? true,
         :count 2027,
         :ok-count 2027,
         :fail-count 0,
         :info-count 0,
         :by-f {:broadcast {:valid? true,
                            :count 966,
                            :ok-count 966,
                            :fail-count 0,
                            :info-count 0},
                :read {:valid? true,
                       :count 1061,
                       :ok-count 1061,
                       :fail-count 0,
                       :info-count 0}}},
 :availability {:valid? true, :ok-fraction 1.0},
 :net {:all {:send-count 41705,
             :recv-count 41655,
             :msg-count 41705,
             :msgs-per-op 20.574741},
       :clients {:send-count 4154, :recv-count 4154, :msg-count 4154},
       :servers {:send-count 37551,
                 :recv-count 37501,
                 :msg-count 37551,
                 :msgs-per-op 18.525408},
       :valid? true},
 :workload {:worst-stale (<TRUNCATED>),
            :duplicated-count 0,
            :valid? true,
            :lost-count 0,
            :lost (),
            :stable-count 966,
            :stale-count 963,
            :stale (<TRUNCATED>),
            :never-read-count 0,
            :stable-latencies {0 0,
                               0.5 608,
                               0.95 1043,
                               0.99 1425,
                               1 1644},
            :attempt-count 966,
            :never-read (),
            :duplicated {}},
 :valid? true}
```

For further improvements on this section of Gossip Glomers, I would definitely like to approach a method which more rapidly
propagates values to prevent stale values. One idea for this might include changing the way topology is defined, rather than
using the predefined topology provided by Fly.io. Another idea might be to prioritize communicating with nodes which haven't 
communicated with a given node for `X` amount of time.