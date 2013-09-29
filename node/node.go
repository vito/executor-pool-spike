package node

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/nu7hatch/gouuid"
)

type Node struct {
	ID string

	heartbeatInterval time.Duration

	starts chan Instance

	registry Registry

	store *etcd.Client
}

func NewNode(store *etcd.Client, heartbeatInterval time.Duration) Node {
	id, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}

	node := Node{
		ID: id.String(),

		heartbeatInterval: heartbeatInterval,

		starts: make(chan Instance),

		registry: NewRegistry(),

		store: store,
	}

	go node.handleStarts()

	if heartbeatInterval != 0 {
		go node.heartbeatRegistry()
	}

	return node
}

func (node Node) StartApp(app string, index int) {
	node.starts <- Instance{app, index, false}
}

func (node Node) StopApp(app string, index int) {
	node.stopInstance(Instance{app, index, false})
}

func (node Node) LogRegistry() {
	instances := node.registry.AllInstances()

	bar := []string{}

	for _, inst := range instances {
		if inst.MarkedForDeath {
			bar = append(bar, "\x1b[31m▇\x1b[0m")
		} else {
			bar = append(bar, "▇")
		}
	}

	fmt.Printf("\x1b[34mrunning\x1b[0m %d: %3d %s\n", os.Getpid(), len(instances), strings.Join(bar, " "))
}

func (node Node) heartbeatRegistry() {
	interval := node.heartbeatInterval

	ttl := uint64(interval * 3 / time.Second)

	for {
		time.Sleep(interval)

		fmt.Println("\x1b[93mheartbeating\x1b[0m")

		for _, inst := range node.registry.AllInstances() {
			node.store.Set(inst.StoreKey(), "ok", ttl)
		}

	}
}

func (node Node) handleStarts() {
	for {
		node.startInstance(<-node.starts)
	}
}

func (node Node) startInstance(instance Instance) {
	instances := node.registry.InstancesOf(instance.App)

	if len(instances) > 0 {
		count := len(instances)

		delay := time.Duration(count) * 10 * time.Millisecond

		fmt.Println("\x1b[33mhesitating\x1b[0m", delay)

		time.Sleep(delay)
	}

	ok := node.volunteer(instance)
	if !ok {
		fmt.Println("\x1b[31mdropped\x1b[0m", instance.Index)
		return
	}

	fmt.Println("\x1b[32mstarted\x1b[0m", instance.Index)

	// make 25% of them crash after a random amount of time
	//
	// because that's more interesting
	if rand.Intn(4) == 0 {
		instance.MarkedForDeath = true

		go func() {
			time.Sleep(time.Duration(5*rand.Intn(10)) * time.Second)
			node.StopApp(instance.App, instance.Index)
		}()
	}

	node.registry.Register(instance)
}

func (node Node) stopInstance(instance Instance) {
	node.registry.Unregister(instance)
	node.store.Delete(instance.StoreKey())
}

func (node Node) volunteer(instance Instance) bool {
	_, ok, err := node.store.TestAndSet(
		instance.StoreKey(),
		"",
		"ok",
		uint64(node.heartbeatInterval*3/time.Second),
	)

	if err != nil {
		return false
	}

	return ok
}
