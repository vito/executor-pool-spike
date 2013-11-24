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
	"github.com/cloudfoundry/yagnats"

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

	existingApps := 0
	allApps, err := store.Get(fmt.Sprintf("/apps/%s", *app), false)
	if err == nil {
		existingApps = len(allApps.Kvs)
	}

	go func() {
		for i := existingApps; i < *instances; i++ {
			start := messages.AppStart{
				Guid:  *app,
				Index: i,
			}

			msg, err := json.Marshal(start)
			if err != nil {
				log.Fatalln(err)
			}

			nats.Publish("app.start", msg)
		}
	}()

	for {
		res, err := store.Get(fmt.Sprintf("/apps/%s", *app), false)
		if err != nil {
			log.Println(err)
			continue
		}

		log.Println("entries:", len(res.Kvs))

		time.Sleep(1 * time.Second)

		if len(res.Kvs) == *instances {
			break
		}
	}

	log.Println("took:", time.Since(start))
}
