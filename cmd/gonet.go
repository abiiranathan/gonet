package main

import (
	"os"

	"github.com/abiiranathan/gonet"
)

func main() {
	gonet.WriteMetrics(os.Stdout)
}
