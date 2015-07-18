package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"
)

var clientChan = ClientChannel{
	clientChans: make(map[int64]chan string),
}

type ClientChannel struct {
	clientChans map[int64]chan string

	mu sync.Mutex
}

func (c *ClientChannel) Set(key int64, ch chan string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.clientChans[key] = ch
}

func (c *ClientChannel) Broadcast(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, ch := range c.clientChans {
		ch <- msg
	}
}

func (c *ClientChannel) Del(key int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.clientChans, key)
}

func adminService() {
	l, err := net.Listen("tcp", ":2000")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func(c net.Conn) {
			var msg string
			for {
				_, err := fmt.Fscanln(c, &msg)
				if err != nil {
					if err.Error() == "EOF" {
						break
					}
					continue
				}
				clientChan.Broadcast(msg)
			}
			c.Close()
		}(conn)
	}
}

func clientService() {
	l, err := net.Listen("tcp", ":2001")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func(c net.Conn) {
			closeCh := make(chan struct{})
			go func(c net.Conn, closeCh chan struct{}) {
				for {
					b := make([]byte, 1)
					_, err := c.Read(b)
					if err != nil {
						closeCh <- struct{}{}
						break
					}
				}
			}(c, closeCh)

			ch := make(chan string)
			key := time.Now().Unix()
			clientChan.Set(key, ch)
			for {
				select {
				case msg := <-ch:
					fmt.Fprintln(c, msg)
				case <-closeCh:
					break
				}
			}
			clientChan.Del(key)
			c.Close()
		}(conn)
	}
}

func main() {
	go func() {
		log.Println(http.ListenAndServe(":8080", nil))
	}()
	go adminService()
	clientService()
}
