package executor

import (
	"fmt"
)

type Instance struct {
	App   string
	Index int

	MarkedForDeath bool
}

func (i Instance) StoreKey() string {
	return fmt.Sprintf("/apps/%s/%d", i.App, i.Index)
}
