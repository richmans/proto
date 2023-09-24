package main

import (
"net"
"fmt"
"os"
"strings"
)

type DatabaseServer struct {
	port uint16
	store map[string]string
}

func NewDatabaseServer(port uint16) *DatabaseServer {
	return &DatabaseServer{
		port,
		make(map[string]string),
	}
}

func (s *DatabaseServer) Run() {
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer pc.Close()
	buf := make([]byte, 1000)
	fmt.Println("Database server listening for packets")
	for {
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			continue
		}
		s.handle(pc, addr, string(buf[:n]))
	}	
}

func (s *DatabaseServer) handle(pc net.PacketConn, addr net.Addr, req string) {
	var resp string
	fmt.Printf(">> %s\n", req)
	if strings.Contains(req, "=") {
		s.handleInsert(req)
	} else {
		resp = s.handleRetrieve(req)
	}
	if resp != "" {
		fmt.Printf("<< %s\n", resp)
		pc.WriteTo([]byte(resp), addr)
	}
}

func (s *DatabaseServer) handleInsert(req string) {
	parts := strings.SplitAfterN(req, "=", 2)
	key := strings.Trim(parts[0], "=")
	value := parts[1]
	if key == "version" {
		return
	}
	s.store[key] = value
}

func (s *DatabaseServer) handleRetrieve(req string) string{
	if req == "version" {
		return "version=Deathstar planetary target database v2.9 build 73839"
	}
	value := s.store[req]
	return fmt.Sprintf("%s=%s", req, value)
}
