package main

import (
"net"
"os"
"fmt"
"encoding/binary"
)

type MeansServer struct {
	port uint16
}

func NewMeansServer(port uint16) *MeansServer {
	return &MeansServer{port}
}

type Price struct {
	Price int32
	Timestamp int32
}
type MeansSession struct {
	prices []Price
}
func NewMeansSession() *MeansSession {
	return &MeansSession{}
}

type MeansRequest struct {
	Type uint8
	Param1 int32
	Param2 int32
}

func (s *MeansServer) Run() {
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	server, err := net.Listen("tcp", addr)
    if err != nil {
      fmt.Println("Error listening:", err.Error())
        os.Exit(1)
    }
    defer server.Close()
    fmt.Println("MeansServer waiting for client...")
    for {
      connection, err := server.Accept()
      if err != nil {
        fmt.Println("Error accepting: ", err.Error())
        os.Exit(1)
      }
      fmt.Println("client connected")
			ses := NewMeansSession()
      go ses.processClient(connection)
		}
}

func (s *MeansSession) processClient(con net.Conn) error {
	var err error
	for {
	  err = s.processRequest(con)
	  if err != nil {
	    break
	  }
	}
	if err != nil {
		fmt.Println(err)
	}
  con.Close()
	return err
}

func (s *MeansSession) processRequest( con net.Conn) error {
	var r MeansRequest
	err := binary.Read(con, binary.BigEndian, &r);
	if err != nil {
    return err
	}
	switch r.Type {
		case 73:
			s.do_insert(r.Param1, r.Param2)
		case 81:
			avg := s.get_average(r.Param1, r.Param2)
			binary.Write(con, binary.BigEndian, &avg)
		default:
			err = fmt.Errorf("invalid type %d", r.Type)
	}
	return err
}

func (s *MeansSession) do_insert(timestamp int32, price int32) {
	p := Price{price,timestamp}
	fmt.Printf("Adding %d %d\n", timestamp, price)
	s.prices = append(s.prices, p)
}

func (s *MeansSession) get_average(start int32, end int32) int32 {
	var count int64 =  0
	var total int64 = 0
	for _, p := range s.prices { 
		if p.Timestamp >= start && p.Timestamp <= end {
			count += 1
			total += int64(p.Price)
		}
	}
	if count == 0 {
		return 0
	}
	result := int32(total / count)
	fmt.Printf("Q %d-%d: %d", start, end, result)
	return result
}