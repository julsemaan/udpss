package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Please provide a host:port string")
		return
	}
	CONNECT := arguments[1]

	s, err := net.ResolveUDPAddr("udp4", CONNECT)
	c, err := net.DialUDP("udp4", nil, s)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("The UDP server is %s\n", c.RemoteAddr().String())
	defer c.Close()

	shutdown := false
	shutdownTicks := 5
	serverHostname := ""
	for {
		if shutdown {
			shutdownTicks -= 1
			if shutdownTicks == 0 {
				fmt.Println("Done")
				return
			}
		}

		_, err = c.Write([]byte(fmt.Sprintf("ping|%s\n", serverHostname)))

		if err != nil {
			fmt.Println(err)
			return
		}

		buffer := make([]byte, 1024)
		n, _, err := c.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println(err)
			return
		}

		reply := strings.Split(string(buffer[0:n]), "|")
		serverReady := (reply[0] == "true")
		serverHostname = reply[1]

		if !serverReady && !shutdown {
			fmt.Println("Server is shutting down, doing", shutdownTicks, "last ticks before stopping the client")
			shutdown = true
		}

		fmt.Printf("ready:%t, hostname:%s\n", serverReady, serverHostname)

		time.Sleep(1 * time.Second)
	}
}
