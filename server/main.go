package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var hostname, _ = os.Hostname()
var online = true

func main() {
	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Please provide a port number!")
		return
	}
	PORT := ":" + arguments[1]

	s, err := net.ResolveUDPAddr("udp4", PORT)
	if err != nil {
		fmt.Println(err)
		return
	}

	connection, err := net.ListenUDP("udp4", s)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer connection.Close()
	buffer := make([]byte, 1024)

	catchShutdown()
	fmt.Println("Ready...")

	for {
		n, addr, err := connection.ReadFromUDP(buffer)
		fmt.Println("-> ", string(buffer[0:n]))

		reply := strings.Split(string(buffer[0:n]), "|")
		clientUUID := reply[0]
		serverHostname := reply[1]

		if serverHostname != "" && serverHostname != hostname {
			fmt.Println("Received a packet that isn't for this server. Will not reply")
			continue
		}

		_, err = connection.WriteToUDP([]byte(fmt.Sprint(clientUUID, "|", online, "|", hostname)), addr)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}

func catchShutdown() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("Stopping")
		online = false
		time.Sleep(20 * time.Second)
		fmt.Println("Stopped")
		os.Exit(0)
	}()
}
