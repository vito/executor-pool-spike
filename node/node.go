package node

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/nu7hatch/gouuid"
)

type Node struct {
	ID string

	starts chan Instance

	registry map[string]map[int]Instance

	store *etcd.Client

	sync.RWMutex
}

func NewNode(store *etcd.Client) Node {
	id, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}

	node := Node{
		ID: id.String(),

		starts: make(chan Instance),

		registry: make(map[string]map[int]Instance),

		store: store,
	}

	go node.handleStarts()

	return node
}

func (node Node) StartApp(app string, index int) {
	node.Lock()
	defer node.Unlock()

	node.starts <- Instance{app, index, false}
}

func (node Node) StopApp(app string, index int) {
	node.Lock()
	defer node.Unlock()

	node.killInstance(Instance{app, index, false})
}

func (node Node) HeartbeatRegistry(interval time.Duration) {
	node.RLock()
	defer node.RUnlock()

	ttl := uint64(interval * 3 / time.Second)

	for {
		time.Sleep(interval)

		fmt.Println("\x1b[93mheartbeating\x1b[0m")

		for _, inst := range node.allInstances() {
			node.store.Set(inst.StoreKey(), "ok", ttl)
		}
	}
}

func (node Node) LogRegistry() {
	node.RLock()
	defer node.RUnlock()

	instances := node.allInstances()

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

func (node Node) handleStarts() {
	for {
		node.startInstance(<-node.starts)
	}
}

func (node Node) startInstance(instance Instance) {
	instances := node.instancesOf(instance.App)

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

	node.registerInstance(instance)
}

func (node Node) instancesOf(app string) []Instance {
	instances := []Instance{}

	indices, found := node.registry[app]
	if !found {
		return instances
	}

	for _, instance := range indices {
		instances = append(instances, instance)
	}

	return instances
}

func (node Node) allInstances() []Instance {
	instances := []Instance{}

	for app, _ := range node.registry {
		instances = append(instances, node.instancesOf(app)...)
	}

	return instances
}

func (node Node) volunteer(instance Instance) bool {
	_, ok, err := node.store.TestAndSet(
		instance.StoreKey(),
		"",
		"ok",
		0,
	)

	if err != nil {
		return false
	}

	return ok
}

func (node Node) registerInstance(instance Instance) {
	instances, found := node.registry[instance.App]

	if found {
		instances[instance.Index] = instance
	} else {
		node.registry[instance.App] = map[int]Instance{instance.Index: instance}
	}
}

func (node Node) killInstance(instance Instance) {
	indices, found := node.registry[instance.App]
	if !found {
		return
	}

	delete(indices, instance.Index)

	node.store.Delete(instance.StoreKey())
}
