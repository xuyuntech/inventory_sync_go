package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/xuyuntech/inventory_sync_go/api"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	a := api.New()
	if err := a.Run(); err != nil {
		logrus.Fatal(err)
	}
}
