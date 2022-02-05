package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("Entering an infinite loop...")
	for {
		fmt.Println("Sleeping 300...")
		time.Sleep(300 * time.Second)
	}
}
