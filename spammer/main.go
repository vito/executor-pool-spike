package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/nu7hatch/gouuid"
	"github.com/vito/yagnats"

	"github.com/vito/executor-pool-spike/messages"
)

var app = flag.String("app", "", "app identifier (default: random guid)")
var instances = flag.Int("instances", 100, "instances to start")

func main() {
	flag.Parse()

	nats := yagnats.NewClient()

	natsInfo := &yagnats.ConnectionInfo{
		Addr: "localhost:4222",
	}

	err := nats.Connect(natsInfo)
	if err != nil {
		log.Fatalln(err)
	}

	if *app == "" {
		guid, err := uuid.NewV4()
		if err != nil {
			log.Fatalln(err)
		}

		*app = guid.String()
	}

	store := etcd.NewClient()

	start := time.Now()

	existing, err := store.Get(fmt.Sprintf("/apps/%s", *app))
	if err != nil {
		log.Println(err)
	}

	go func() {
		for i := len(existing); i < *instances; i++ {
			start := messages.AppStart{
				Guid:  *app,
				Index: i,
			}

			msg, err := json.Marshal(start)
			if err != nil {
				log.Fatalln(err)
			}

			nats.Publish("app.start", string(msg))
		}
	}()

	for {
		res, err := store.Get(fmt.Sprintf("/apps/%s", *app))
		if err != nil {
			log.Println(err)
		}

		log.Println("entries:", len(res))

		if len(res) == *instances {
			break
		}
	}

	log.Println("took:", time.Since(start))
}
