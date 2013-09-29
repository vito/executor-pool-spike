package hero

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/coreos/etcd/store"
	"github.com/coreos/go-etcd/etcd"

	"github.com/vito/executor-pool-spike/node"
)

func SaveLives(etcd *etcd.Client, node node.Node) {
	deadInstances := make(chan *store.Response)
	stop := make(chan bool)

	go func() {
		for {
			change := <-deadInstances

			if change.Action != "DELETE" {
				continue
			}

			go resurrect(node, change.Key)
		}
	}()

	_, err := etcd.Watch("/apps", 0, deadInstances, stop)
	if err != nil {
		panic(err)
		return
	}
}

func resurrect(node node.Node, key string) {
	fmt.Println("\x1b[91mCRASH!\x1b[0m", key)

	path := strings.Split(key, "/")

	index, err := strconv.Atoi(path[3])
	if err != nil {
		fmt.Println("non-numeric index:", path[3])
		return
	}

	node.StartApp(path[2], index)
}
