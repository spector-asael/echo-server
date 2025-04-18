package main

import (
	"fmt"
	"net"
	"sync"
)

func handleConnection(conn net.Conn, wg *sync.WaitGroup) { // Function to handle connections
	defer wg.Done()
	defer conn.Close()

	buf := make([]byte, 1024) // Can handle up to 1024 bytes

	for { // Infinite loop to read and write from clients (echoing messages back)
		n, err := conn.Read((buf)) // Read from client
		if err != nil {
			fmt.Println("Error reading from client:", err)
			return
		}

		_, err = conn.Write(buf[:n]) // Write from client
		if err != nil {
			fmt.Println("Error writing to client:", err)
			return
		}
	}
}

func main() {
	port := ":4000"
	listener, err := net.Listen("tcp", port) // Set up a tcp connection on port 4000

	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup

	defer listener.Close() // Close connection at the end of the program

	fmt.Printf("Server listening on %s\n", port)
	for { // Infinite loop to accept multiple connections
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting:", err)
			continue
		}
		wg.Add(1)
		go handleConnection(conn, &wg) // Function to handle connections
	}

	wg.Wait()
}
