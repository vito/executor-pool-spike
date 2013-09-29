package node

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/coreos/etcd/store"
	"github.com/coreos/go-etcd/etcd"
	"github.com/nu7hatch/gouuid"
)

type Node struct {
	ID string

	// map from app IDs to indexes
	registry map[string][]int

	starts chan Instance

	store *etcd.Client
}

func NewNode(store *etcd.Client) Node {
	id, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}

	node := Node{
		ID: id.String(),

		registry: map[string][]int{},

		starts: make(chan Instance),

		store: store,
	}

	go node.handleStarts()

	return node
}

func (node Node) StartApp(app string, index int) {
	node.starts <- Instance{app, index}
}

func (node Node) LogRegistry() {
	for app, _ := range node.registry {
		instances, err := node.instancesOf(app)
		if err != nil {
			continue
		}

		bar := []string{}

		for _, inst := range instances {
			if inst.TTL != 0 {
				bar = append(bar, "\x1b[31m▇\x1b[0m")
			} else {
				bar = append(bar, "▇")
			}
		}

		fmt.Printf("\x1b[34mrunning\x1b[0m %s: %3d %s\n", app, len(instances), strings.Join(bar, " "))
	}
}

func (node Node) handleStarts() {
	for {
		node.startInstance(<-node.starts)
	}
}

func (node Node) startInstance(instance Instance) {
	instances, err := node.instancesOf(instance.App)

	if err == nil {
		count := len(instances)

		delay := time.Duration(count) * 10 * time.Millisecond

		fmt.Println("\x1b[33mhesitating\x1b[0m", delay)

		time.Sleep(delay)
	}

	var lifespan uint64

	// make 25% of them crash after a random amount of time
	//
	// because that's more interesting
	if rand.Intn(4) == 0 {
		lifespan = uint64(5 * rand.Intn(10))
	} else {
		lifespan = 0
	}

	ok := node.volunteer(instance, lifespan)
	if !ok {
		fmt.Println("\x1b[31mdropped\x1b[0m", instance.Index)
		return
	}

	fmt.Println("\x1b[32mstarted\x1b[0m", instance.Index)

	node.registerInstance(instance, lifespan)
}

func (node Node) instancesOf(app string) ([]*store.Response, error) {
	return node.store.Get(fmt.Sprintf("/node/%s/apps/%s", node.ID, app))
}

func (node Node) volunteer(instance Instance, ttl uint64) bool {
	_, ok, err := node.store.TestAndSet(
		instance.StoreKey(),
		"",
		"ok",
		ttl,
	)

	if err != nil {
		return false
	}

	return ok
}

func (node Node) registerInstance(instance Instance, ttl uint64) {
	ownerKey := fmt.Sprintf("/node/%s/apps/%s/%d", node.ID, instance.App, instance.Index)

	_, err := node.store.Set(ownerKey, "ok", ttl)
	if err != nil {
		fmt.Println("error setting owner:", err)
	}

	indices, found := node.registry[instance.App]
	if found {
		node.registry[instance.App] = append(indices, instance.Index)
	} else {
		node.registry[instance.App] = []int{instance.Index}
	}
}
