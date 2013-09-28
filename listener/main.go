package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/vito/yagnats"

	"github.com/vito/executor-pool-spike/node"
)

type appStart struct {
	Guid  string `json:"guid"`
	Index int    `json:"index"`
}

func main() {
	store := etcd.NewClient()

	nats := yagnats.NewClient()

	natsInfo := &yagnats.ConnectionInfo{
		Addr: "localhost:4222",
	}

	err := nats.Connect(natsInfo)
	if err != nil {
		log.Fatalln(err)
	}

	node := node.NewNode(store)

	_, err = nats.Subscribe("app.start", func(msg *yagnats.Message) {
		var startMsg appStart
		err := json.Unmarshal([]byte(msg.Payload), &startMsg)
		if err != nil {
			log.Println("failed to unmarshal", msg.Payload)
			return
		}

		node.StartApp(startMsg.Guid, startMsg.Index)
	})

	if err != nil {
		log.Fatalln(err)
	}

	for {
		time.Sleep(1 * time.Second)
		node.LogRegistry()
	}
}
