package node

import (
	"fmt"
)

type Instance struct {
	App   string
	Index int
}

func (i Instance) StoreKey() string {
	return fmt.Sprintf("/apps/%s/%d", i.App, i.Index)
}
