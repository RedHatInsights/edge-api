package main

import (
	"fmt"
	"time"
)

func main() {

	for {
		fmt.Println("Sleeping 300...")
		time.Sleep(300 * time.Second)
	}
}
