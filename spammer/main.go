package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/nu7hatch/gouuid"
	"github.com/vito/yagnats"
)

type appStart struct {
	Guid  string `json:"guid"`
	Index int    `json:"index"`
}

func main() {
	nats := yagnats.NewClient()

	natsInfo := &yagnats.ConnectionInfo{
		Addr: "localhost:4222",
	}

	err := nats.Connect(natsInfo)
	if err != nil {
		log.Fatalln(err)
	}

	guid, err := uuid.NewV4()
	if err != nil {
		log.Fatalln(err)
	}

	store := etcd.NewClient()

	start := time.Now()

	go func() {
		for i := 0; i < 100; i++ {
			start := appStart{
				Guid:  guid.String(),
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
		res, err := store.Get(fmt.Sprintf("/apps/%s", guid))
		if err != nil {
			log.Println(err)
		}

		log.Println("entries:", len(res))

		if len(res) == 100 {
			break
		}
	}

	log.Println("took:", time.Since(start))
}
