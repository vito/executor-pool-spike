package main

import (
	"encoding/json"
	"flag"
	"log"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/vito/yagnats"

	"github.com/vito/executor-pool-spike/executor"
	"github.com/vito/executor-pool-spike/hero"
	"github.com/vito/executor-pool-spike/messages"
	"github.com/vito/executor-pool-spike/starter"
)

var heartbeatInterval = flag.Int("heartbeatInterval", 10, "heartbeat interval")

func main() {
	flag.Parse()

	store := etcd.NewClient()

	nats := yagnats.NewClient()

	natsInfo := &yagnats.ConnectionInfo{
		Addr: "localhost:4222",
	}

	err := nats.Connect(natsInfo)
	if err != nil {
		log.Fatalln(err)
	}

	node := executor.NewNode(store, time.Duration(*heartbeatInterval)*time.Second)

	starter := starter.NewStarter(node)

	go hero.SaveLives(store, starter)

	_, err = nats.Subscribe("app.start", func(msg *yagnats.Message) {
		var startMsg messages.AppStart
		err := json.Unmarshal([]byte(msg.Payload), &startMsg)
		if err != nil {
			log.Println("failed to unmarshal", msg.Payload)
			return
		}

		starter.Start(startMsg.Guid, startMsg.Index)
	})

	if err != nil {
		log.Fatalln(err)
	}

	for {
		time.Sleep(1 * time.Second)
		node.LogRegistry()
	}
}
