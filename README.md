# Executor Pool Experimentation

This is a bunch of spiked code with the intention of designing
a self-organizing network of Executors for use in Cloud Foundry.

## Running

```
mkdir gospace/
cd gospace/
export GOPATH=$PWD
git clone git@github.com:vito/executor-pool-spike.git src/github.com/vito/executor-pool-spike
cd src/github.com/vito/executor-pool-spike
go get -v ./...

# start 10 nodes
foreman start

# in another tab:
go run spammer/main.go

# check the distribution in the logs of 'foreman start'
```

## Why

Currently the Executors (the DEAs) must constantly advertise their capacity
and app placement, and it is left to orchestration components (the CC) to
determine where to place each instance of an app they want to start.

Ideally, the orchestrator is dead simple, and just publishes its intent:
"start an instance", and relies on the system to do the heavy lifting. This
means the business logic of app placement and load balancing has to be done
elsewhere.

With this logic in the pool of Executor nodes itself, it becomes much simpler
to model rolling deploys. Executors can evacuate all of their applications to
the rest of the nodes in the pool, without relying on a rube goldberg machine
(NATS -> Health Manager -> NATS -> Cloud Controller -> CCDB -> Placement Logic
-> NATS).

It also becomes simple to model resurrection of crashed applications; when an
app goes down another node picks it up.

This is an experiment in self-organizing and loosely-coupled
Executor networks to achieve this goal.

The intention is to come up with a simple, intuitive model that accomplishes
three things:

1. Application instances spread across many nodes
2. Rough balancing of overall application instances
3. Still fast enough to start 100 instances of a single app in a jiffy


## Strategy

The current strategy is to have the Executors record their intention to start
an instance as soon as they try, by registering an entry in etcd.

### Glossary

* `volunteering` - a node writing their intention to start an instance
* `hesitating` - a node waiting before volunteering, to achieve load balancing
  and spreading of application instances

Volunteering is defined as setting a value at the key `/apps/<guid>/<index>`.
In the simplest model, all nodes try to set the key, and the one that actually
wrote it knows that he's the one to start it. This is possible because etcd
returns a `NewKey` value in the response; this should only be `true` for one
of them. The rest just drop the request on the floor.

Hesitating is a concept used for balancing instances; one approach is for
a node that knows he already has 10 instances of an app to wait 10 seconds
before volunteering. This leads to even balancing in the pool at the cost of
longer start times for more instances.


## Initial Findings

Hesitation is the most sensitive part. If you wait too long, it takes minutes
to start an app. If you don't wait long enough, your instances are less evenly
balanced.

100 instances, waiting 1 second per instance:

```
distribution:
12:04:02 node2.1  | 2013/09/28 12:04:02 running 10
12:04:03 node1.1  | 2013/09/28 12:04:03 running 10
12:04:03 node4.1  | 2013/09/28 12:04:03 running 12
12:04:03 node5.1  | 2013/09/28 12:04:03 running 9
12:04:03 node3.1  | 2013/09/28 12:04:03 running 9
12:04:03 node6.1  | 2013/09/28 12:04:03 running 9
12:04:03 node7.1  | 2013/09/28 12:04:03 running 11
12:04:03 node8.1  | 2013/09/28 12:04:03 running 11
12:04:03 node10.1 | 2013/09/28 12:04:03 running 9
12:04:03 node9.1  | 2013/09/28 12:04:03 running 10

time to start: 8m10.312597514s
```

100 instances, waiting 10 milliseconds per instance:

```
distribution:
12:05:15 node2.1  | 2013/09/28 12:05:15 running 12
12:05:15 node1.1  | 2013/09/28 12:05:15 running 10
12:05:15 node4.1  | 2013/09/28 12:05:15 running 8
12:05:15 node3.1  | 2013/09/28 12:05:15 running 9
12:05:15 node5.1  | 2013/09/28 12:05:15 running 13
12:05:15 node7.1  | 2013/09/28 12:05:15 running 10
12:05:15 node6.1  | 2013/09/28 12:05:15 running 10
12:05:15 node8.1  | 2013/09/28 12:05:15 running 8
12:05:15 node9.1  | 2013/09/28 12:05:15 running 10
12:05:15 node10.1 | 2013/09/28 12:05:15 running 10

time to start: 5.319348414s
```

As you can see 10 milliseconds seems to be enough to achieve the 3 goals:
quick startup time, spread across many nodes, with roughly even distribution.

Also, even with the most naive approach of sleeping 1 second per instance, you
at least immediately have 1 instance starting on every node. This means if you
have 10 nodes you can start 10 instances immediately. The rest "trickle in"
over time.


## Next Steps

### Rate-Limiting & Preventing Start Storms

"Hesitation" becomes a natural place to rate-limit start requests that are
performed by the Executor, to improve stability and remove the possiblity of
a storm of start requests taking down your nodes.

### Evacuation & Crash Recovery

If in the act of volunteering, the node wrote to the key enough information
for other nodes to start the same instance (like the message the node
responded to), this can be trivially extended for evacuation and crashed
instances.

Other nodes watch the path that the nodes write to, and respond to DELETE
events. If an entry disappeared while in a RUNNING state, all other nodes
treat it as if a start was requested: they perform the same hesitation and
volunteering flow as starting instances.

The key can be given a TTL and as long as a node is running an instance, he
keeps bumping it.

If the instance crashes or the node is being evacuated, the node running the
instance deletes its key.

If an instance is shut down by the user, the node writes STOPPED state to the
key, so other nodes know not to volunteer (a node seeing a DELETE event will
see the value of the key), and then deletes it.

If a node hard-crashes, the TTL will expire and a DELETE event will trigger.
