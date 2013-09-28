package node

import (
	"fmt"
	"strings"
	"time"

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

type Instance struct {
	App   string
	Index int
}

func (i Instance) StoreKey() string {
	return fmt.Sprintf("/apps/%s/%d", i.App, i.Index)
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

func (node Node) handleStarts() {
	for {
		instance := <-node.starts

		indices, found := node.registry[instance.App]
		if found {
			delay := time.Duration(len(indices)) * time.Millisecond

			fmt.Println("\x1b[33mhesitating\x1b[0m", delay)

			time.Sleep(delay)
		}

		ok := node.volunteer(instance)
		if !ok {
			fmt.Println("\x1b[31mdropped\x1b[0m", instance.Index)
			continue
		}

		fmt.Println("\x1b[32mstarted\x1b[0m", instance.Index)

		node.registerInstance(instance)
	}
}

func (node Node) LogRegistry() {
	for _, indices := range node.registry {
		fmt.Printf("\x1b[34mrunning\x1b[0m %3d: %s\n", len(indices), strings.Repeat("▇", len(indices)))
	}
}

func (node Node) volunteer(instance Instance) bool {
	res, err := node.store.Set(instance.StoreKey(), "ok", 0)
	if err != nil {
		fmt.Println("error setting key:", err)
		return false
	}

	return res.NewKey
}

func (node Node) registerInstance(instance Instance) {
	indices, found := node.registry[instance.App]
	if found {
		node.registry[instance.App] = append(indices, instance.Index)
	} else {
		node.registry[instance.App] = []int{instance.Index}
	}
}
