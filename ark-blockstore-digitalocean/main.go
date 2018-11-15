package main

import (
	arkplugin "github.com/heptio/ark/pkg/plugin"
	"github.com/sirupsen/logrus"
)

func main() {
	arkplugin.NewServer(arkplugin.NewLogger()).
		RegisterBlockStore("digitalocean-blockstore", newBlockStore).
		Serve()
}

func newBlockStore(logger logrus.FieldLogger) (interface{}, error) {
	return &BlockStore{FieldLogger: logger}, nil
}
