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