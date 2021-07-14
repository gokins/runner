package cmd

import (
	"context"
)

var (
	Ctx  context.Context
	cncl context.CancelFunc
)

func init() {
	Ctx, cncl = context.WithCancel(context.Background())
}
func Cancel() {
	if cncl != nil {
		cncl()
	}
}
