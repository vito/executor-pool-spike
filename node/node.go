package node

import (
	"fmt"
	"log"
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
			log.Println("waiting", len(indices), "seconds")
			time.Sleep(time.Duration(len(indices)) * 10 * time.Millisecond)
		}

		log.Println("volunteering", instance)
		ok := node.volunteer(instance)
		if !ok {
			log.Println("failed", instance)
			// someone else got it
			continue
		}

		log.Println("started!", instance)

		node.registerInstance(instance)
	}
}

func (node Node) LogRegistry() {
	for app, indices := range node.registry {
		log.Println("running", app, len(indices))
	}
}

func (node Node) volunteer(instance Instance) bool {
	res, err := node.store.Set(instance.StoreKey(), "ok", 0)
	if err != nil {
		log.Println("Error setting key:", err)
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
