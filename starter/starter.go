package starter

import (
	"fmt"
	"sync"
	"time"

	"github.com/mgutz/ansi"

	"github.com/vito/executor-pool-spike/executor"
)

type Starter struct {
	node executor.Node

	pipelines map[string]chan executor.Instance

	sync.RWMutex
}

func NewStarter(node executor.Node) *Starter {
	return &Starter{
		node: node,

		pipelines: make(map[string]chan executor.Instance),
	}
}

func (s *Starter) Start(app string, index int) {
	s.pipelineFor(app) <- executor.Instance{app, index, false}
}

func (s *Starter) pipelineFor(app string) chan executor.Instance {
	s.Lock()
	defer s.Unlock()

	pipeline, found := s.pipelines[app]

	if !found {
		pipeline = make(chan executor.Instance)
		s.pipelines[app] = pipeline

		go s.dispatchStarts(app, pipeline)
	}

	return pipeline
}

func (s *Starter) closePipe(app string) {
	s.Lock()
	defer s.Unlock()

	pipeline, found := s.pipelines[app]
	if !found {
		return
	}

	close(pipeline)
	delete(s.pipelines, app)
}

func (s *Starter) dispatchStarts(app string, pipeline chan executor.Instance) {
	for {
		select {
		case instanceToStart := <-pipeline:
			fmt.Println(ansi.Color("dispatching start", "cyan"), app, instanceToStart.Index)
			s.node.StartInstance(instanceToStart)

		case <-time.After(10 * time.Second):
			fmt.Println(ansi.Color("pipeline expired", "red"), app)
			s.closePipe(app)
			return
		}
	}
}
