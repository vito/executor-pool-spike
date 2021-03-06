package executor

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/mgutz/ansi"
	"github.com/nu7hatch/gouuid"
)

type Node struct {
	ID string

	heartbeatInterval time.Duration

	registry *Registry

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

		registry: NewRegistry(),

		store: store,
	}

	if heartbeatInterval != 0 {
		go node.heartbeatRegistry()
	}

	return node
}

func (node Node) StartInstance(instance Instance) {
	stolen := node.hesitate(instance)
	if stolen {
		fmt.Println(ansi.Color("yoinked", "yellow"), instance.Index)
		return
	}

	ok := node.volunteer(instance)
	if !ok {
		fmt.Println(ansi.Color("dropped", "red"), instance.Index)
		return
	}

	fmt.Println(ansi.Color("started", "green"), instance.Index)

	// make 25% of them crash after a random amount of time
	//
	// because that's more interesting
	if rand.Intn(4) == 0 {
		instance.MarkedForDeath = true

		go func() {
			time.Sleep(time.Duration(5+rand.Intn(10)) * time.Second)
			node.StopInstance(instance)
		}()
	}

	node.registry.Register(instance)
}

func (node Node) StopInstance(instance Instance) {
	node.registry.Unregister(instance)
	node.store.Delete(instance.StoreKey(), true)
}

func (node Node) LogRegistry() {
	instances := node.registry.AllInstances()

	bar := []string{}

	for _, inst := range instances {
		if inst.MarkedForDeath {
			bar = append(bar, ansi.Color("▇", "red"))
		} else {
			bar = append(bar, "▇")
		}
	}

	fmt.Printf("%s %d: %3d %s\n", ansi.Color("running", "blue"), os.Getpid(), len(instances), strings.Join(bar, ""))
}

func (node Node) hesitate(instance Instance) bool {
	instances := node.registry.InstancesOf(instance.App)

	stolen := make(chan *etcd.Response)

	delay := 10 * time.Duration(len(instances)) * time.Millisecond

	fmt.Println(ansi.Color("hesitating", "yellow"), instance.Index, delay)

	go node.store.Watch(instance.StoreKey(), 0, true, stolen, nil)

	select {
	case <-stolen:
		return true
	case <-time.After(delay):
		return false
	}
}

func (node Node) volunteer(instance Instance) bool {
	_, err := node.store.Create(
		instance.StoreKey(),
		"ok",
		uint64(node.heartbeatInterval*3/time.Second),
	)

	return err == nil
}

func (node Node) heartbeatRegistry() {
	interval := node.heartbeatInterval

	ttl := uint64(interval * 3 / time.Second)

	for {
		time.Sleep(interval)

		fmt.Println(ansi.Color("heartbeating", "yellow"))

		for _, inst := range node.registry.AllInstances() {
			node.store.Set(inst.StoreKey(), "ok", ttl)
		}

	}
}
