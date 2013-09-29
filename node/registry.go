package node

import (
	"sync"
)

type Registry struct {
	registry map[string]map[int]Instance

	sync.RWMutex
}

func NewRegistry() Registry {
	return Registry{
		registry: make(map[string]map[int]Instance),
	}
}

func (r Registry) Register(instance Instance) {
	r.Lock()
	defer r.Unlock()

	instances, found := r.registry[instance.App]

	if found {
		instances[instance.Index] = instance
	} else {
		r.registry[instance.App] = map[int]Instance{instance.Index: instance}
	}
}

func (r Registry) Unregister(instance Instance) {
	r.Lock()
	defer r.Unlock()

	indices, found := r.registry[instance.App]
	if !found {
		return
	}

	delete(indices, instance.Index)
}

func (r Registry) InstancesOf(app string) []Instance {
	r.RLock()
	defer r.RUnlock()

	instances := []Instance{}

	indices, found := r.registry[app]
	if !found {
		return instances
	}

	for _, instance := range indices {
		instances = append(instances, instance)
	}

	return instances
}

func (r Registry) AllInstances() []Instance {
	r.RLock()
	defer r.RUnlock()

	instances := []Instance{}

	for _, indices := range r.registry {
		for _, instance := range indices {
			instances = append(instances, instance)
		}
	}

	return instances
}
