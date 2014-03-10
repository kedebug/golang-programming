package main

import (
	"fmt"
	"math/rand"
	"time"
)

func boring(msg string) <-chan string {
	c := make(chan string)
	go func() {
		for i := 0; ; i++ {
			c <- fmt.Sprintf("%s %d", msg, i)
			time.Sleep(time.Duration(rand.Intn(1e3)) * time.Millisecond)
		}
	}()
	return c
}

func fanIn(in1, in2 <-chan string) <-chan string {
	c := make(chan string)
	go func() {
		for {
			select {
			case c <- <-in1:
			case c <- <-in2:
			}
		}
	}()
	return c
}

func main() {
	c := fanIn(boring("joe"), boring("ann"))
	for i := 0; i < 5; i++ {
		fmt.Printf("You say: %q\n", <-c)
	}
	fmt.Println("You are boring, I'm leaving.")
}
