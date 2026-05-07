package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
)

type Message struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

func main() {
	listenAddr := flag.String("listen", "127.0.0.1:7357", "tcp listen address")
	flag.Parse()

	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nview listen error: %v\n", err)
		os.Exit(1)
	}
	defer ln.Close()

	fmt.Printf("nview listening on %s\n", *listenAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr, "accept error: %v\n", err)
			continue
		}

		go handle(conn)
	}
}

func handle(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	encoder := json.NewEncoder(conn)

	for scanner.Scan() {
		line := scanner.Bytes()
		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		fmt.Printf("recv %s\n", msg.Type)

		if msg.Type == "hello" {
			_ = encoder.Encode(Message{
				Type: "ack",
				Payload: map[string]any{
					"ok": true,
				},
			})
		}
	}
}
