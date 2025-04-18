package main

import (
	"fmt"
	"net"
)

func handleConnection(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 1024)

	for {
		n, err := conn.Read((buf))
		if err != nil {
			fmt.Println("Error reading from client:", err)
			return
		}

		_, err = conn.Write(buf[:n])
		if err != nil {
			fmt.Println("Error writing to client:", err)
			return
		}
	}
}

func main() {
	port := ":4000"
	listener, err := net.Listen("tcp", port)

	if err != nil {
		panic(err)
	}

	defer listener.Close()

	fmt.Printf("Server listening on %s\n", port)
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting:", err)
			continue
		}
		handleConnection(conn)
	}
}
