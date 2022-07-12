package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
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
	clientUUID := uuid.New().String()
	for {
		if shutdown {
			shutdownTicks -= 1
			if shutdownTicks == 0 {
				fmt.Println("Done")
				return
			}
		}

		_, err = c.Write([]byte(fmt.Sprintf("%s|%s", clientUUID, serverHostname)))

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
		replyUUID := reply[0]
		serverReady := (reply[1] == "true")
		serverHostname = reply[2]

		if clientUUID != replyUUID {
			fmt.Println("Received a UUID that doesn't match the one we sent")
			os.Exit(1)
		}

		if !serverReady && !shutdown {
			fmt.Println("Server is shutting down, doing", shutdownTicks, "last ticks before stopping the client")
			shutdown = true
		}

		fmt.Printf("uuid:%s, ready:%t, hostname:%s\n", replyUUID, serverReady, serverHostname)

		time.Sleep(1 * time.Second)
	}
}
