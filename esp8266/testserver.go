package main

import (
        "fmt"
        "net"
        "os"
        "time"
		"math/rand"
)

func main() {
        arguments := os.Args
        if len(arguments) == 1 {
                fmt.Println("Please provide port number")
                return
        }

        PORT := ":" + arguments[1]
        l, err := net.Listen("tcp", PORT)
        if err != nil {
                fmt.Println(err)
                return
        }
        defer l.Close()
		for {
			c, err := l.Accept()
			if err != nil {
					fmt.Println(err)
					return
			}
			
			go newClient(c)
		}
}

func random(min int, max int) int {
	rand.Seed(time.Now().Unix())
	return rand.Intn(max-min) + min
}

func newClient(c net.Conn) {
	fmt.Println("New client connected")
	var p1 = 0
	var p2 = 0
	var p3 = 0

	for {
		p1 = random(0, 1023)
		p2 = random(0, 1023)
		p3 = random(0, 1023)
		pdata := fmt.Sprintf("%d|%d|%d\r\n", p1, p2, p3)
		c.Write([]byte(pdata))
		time.Sleep(1 * time.Second)
	}
}
