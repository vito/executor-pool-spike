package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/nu7hatch/gouuid"
	"github.com/vito/yagnats"

	"github.com/vito/executor-pool-spike/messages"
)

var app = flag.String("app", "", "app identifier (default: random guid)")
var instances = flag.Int("instances", 100, "instances to start")

var natsAddr = flag.String("natsAddr", "localhost:4222", "NATS server address")
var natsUser = flag.String("natsUser", "", "NATS server username")
var natsPass = flag.String("natsPass", "", "NATS server password")

var etcdCluster = flag.String("etcdCluster", "http://127.0.0.1:4001", "ETCD servers (comma-separated)")

func main() {
	flag.Parse()

	nats := yagnats.NewClient()

	natsInfo := &yagnats.ConnectionInfo{
		Addr:     *natsAddr,
		Username: *natsUser,
		Password: *natsPass,
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

	store := etcd.NewClient(strings.Split(*etcdCluster, ","))

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
