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

# start nats
nats-server -d

# start etcd
etcd

# start 10 nodes
foreman start

# in another tab, send 100 app.starts:
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

Hesitating is a technique used for balancing instances without having to know
the layout of the rest of the pool. For example, a node that already has 10
instances of an app can know to wait 10 seconds before volunteering. If every
node does this, application instances will naturally balance themselves across
the pool, at the cost of longer start times for higher instance counts.

With one node, starting 100 instances will have the following flow (request
handling over time moves down, n is some time scale like milliseconds):

```
-> sleep 0 * n, volunteer for 1, start
-> sleep 1 * n, volunteer for 2, start
-> sleep 2 * n, volunteer for 3, start
-> ...
-> sleep 99 * n, volunteer for 100, start
```

As you can tell, this is not optimal, but it at least rate limits the 100
starts, and the node won't fall over under load.

Adding another node dramatically reduces the time it takes, as they'll
effectively halve the instances they have and thus halve the amount of time
they hesitate.

```
A: sleep 0 * n, volunteer for 1, start
B: sleep 0 * n, volunteer for 1, fail

A: sleep 1 * n, volunteer for 2, fail
B: sleep 0 * n, volunteer for 2, start

A: sleep 1 * n, volunteer for 3, start
B: sleep 1 * n, volunteer for 3, fail

A: sleep 2 * n, volunteer for 4, fail
B: sleep 1 * n, volunteer for 4, start

A: sleep 2 * n, volunteer for 5, start
B: sleep 2 * n, volunteer for 5, fail

A: sleep 3 * n, volunteer for 6, fail
B: sleep 2 * n, volunteer for 6, start

...

A: sleep 50 * n, volunteer for 100, fail
B: sleep 49 * n, volunteer for 100, start
```

## Initial Findings

Hesitation is the most sensitive part. If you wait too long, it takes minutes
to start an app. If you don't wait long enough, your instances may be less
evenly balanced, and more subject to network latency.

100 instances, waiting 1 second per instance:

```
distribution:
node2.1  | running 10
node1.1  | running 10
node4.1  | running 12
node5.1  | running 9
node3.1  | running 9
node6.1  | running 9
node7.1  | running 11
node8.1  | running 11
node10.1 | running 9
node9.1  | running 10

time to start: 8m10.312597514s
```

100 instances, waiting 10 milliseconds per instance:

```
distribution:
node2.1  | running 12
node1.1  | running 10
node4.1  | running 8
node3.1  | running 9
node5.1  | running 13
node7.1  | running 10
node6.1  | running 10
node8.1  | running 8
node9.1  | running 10
node10.1 | running 10

time to start: 5.319348414s
```

100 instances, waiting 1 millisecond per instance:

```
distribution:
node4.1  | running 10
node3.1  | running 10
node1.1  | running 12
node2.1  | running 9
node5.1  | running 10
node6.1  | running 8
node7.1  | running 10
node10.1 | running 10
node8.1  | running 12
node9.1  | running 9

time to start: 735.750816ms
```

At least on a local machine, sleeping for smaller amounts of time does not
appear to adversely affect distribution, and has the most reward. However
once you're down in the millisecond range this may become subject to network
latency.

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
