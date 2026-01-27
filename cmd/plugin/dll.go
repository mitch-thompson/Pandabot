package main

import "C"

import (
	"fmt"
	"log"
	"net"
	"time"
)

var conn net.Conn
var connected bool

func main() {

}

//export GoInit
func GoInit() {
	log.Println("PandaBot DLL loaded - MVP: Attempting connection")

	go connectLoop() // Minimal: Just connect, no further logic
}

//export GoUnload
func GoUnload() {
	if connected {
		conn.Close()
	}
	log.Println("PandaBot DLL unloaded")
}

// connectLoop handles TCP connection (minimal for MVP)
func connectLoop() {
	for {
		if !connected {
			var err error
			conn, err = net.DialTimeout("tcp", "127.0.0.1:31337", time.Second*5)
			if err != nil {
				log.Printf("Connection failed: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			connected = true
			log.Println("Connected to server (MVP)")
			// For MVP, send a hello message
			fmt.Fprintln(conn, "HELLO|MVP|"+time.Now().Format(time.RFC3339))
		}
		time.Sleep(1 * time.Second)
	}
}
