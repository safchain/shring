package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/safchain/shring/pkg/shring"
)

func main() {
	cm := flag.Bool("client", false, "client mode")
	flag.Parse()

	if *cm {
		client, err := shring.NewClient("/dev/shm/ring", 0755, "/tmp/ring.sock", 100)
		if err != nil {
			panic(err)
		}
		blockCh, lostCh := client.Read()

		for {
			select {
			case l := <-lostCh:
				fmt.Printf("Reader: lost %d\n", l)
			case b := <-blockCh:
				fmt.Printf("Reader: %s; %v\n", string(b.Data), b.Err)
			}
		}
	} else {
		server, err := shring.NewServer("/dev/shm/ring", 0755, "/tmp/ring.sock", 100)
		if err != nil {
			panic(err)
		}

		server.Start(context.Background())

		var i int

		for {
			data := fmt.Sprintf("hello %d", i)
			server.Write([]byte(data))
			time.Sleep(time.Millisecond)

			i++
		}
	}
}
