package hero

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/mgutz/ansi"

	"github.com/vito/executor-pool-spike/node"
)

func SaveLives(etcd *etcd.Client, node node.Node) {
	since := uint64(0)

	for {
		change, err := etcd.Watch("/apps", since, nil, nil)
		if err != nil {
			fmt.Println(ansi.Color("watch failed; resting up", "red"))
			time.Sleep(1 * time.Second)
			continue
		}

		since = change.Index + 1

		if change.Action == "DELETE" {
			go resurrect(node, change.Key)
		}
	}
}

func resurrect(node node.Node, key string) {
	fmt.Println(ansi.Color("CRASH!", "red+B:white+h"), key)

	path := strings.Split(key, "/")

	index, err := strconv.Atoi(path[3])
	if err != nil {
		fmt.Println("non-numeric index:", path[3])
		return
	}

	node.StartApp(path[2], index)
}
