package main

import (
"net"
"os"
"fmt"
"bufio"
"encoding/json"
"math/big"
"math"
)

type PrimeRequest struct {
	Method string
	Number *float64
}

type PrimeResponse struct {
	Method string `json:"method"`
	Prime bool `json:"prime"`
}

type PrimeServer struct {
	port uint16
}

func NewPrimeServer(port uint16) *PrimeServer {
	return &PrimeServer{port}
}

func (s *PrimeServer) Run() {
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	server, err := net.Listen("tcp", addr)
    if err != nil {
      fmt.Println("Error listening:", err.Error())
        os.Exit(1)
    }
    defer server.Close()
    fmt.Println("PrimeServer waiting for client...")
    for {
      connection, err := server.Accept()
      if err != nil {
        fmt.Println("Error accepting: ", err.Error())
        os.Exit(1)
      }
      fmt.Println("client connected")
      go s.processClient(connection)
		}
}

func (s *PrimeServer) processClient(con net.Conn) {
	var err error
	scanner := bufio.NewScanner(con)
	for scanner.Scan() {
	  data := scanner.Bytes()
	  fmt.Printf("Received %d bytes\n", len(data))
    err = s.processRequest(data, con)
		if err != nil {
			break
		}
	}
	if scanner.Err() != nil {
    err = scanner.Err()
 	}
	if err != nil {	con.Write([]byte("malformed"))		
		fmt.Println(err)
	}
	fmt.Println("close")
  con.Close()
}
func (s *PrimeServer) processRequest(data []byte, con net.Conn) error {
	var q PrimeRequest
	var r PrimeResponse
	err := json.Unmarshal(data, &q)
	fmt.Println(string(data))
	if err != nil {
		return err
	}
	if q.Method != "isPrime" {
		return fmt.Errorf("wrong method: %s", q.Method)
	}
	if q.Number == nil {
		return fmt.Errorf("no number")
	}
	r.Method = q.Method
	r.Prime = isPrime(*q.Number)
	result, err := json.Marshal(r)
	if err != nil {
		return err
	}
	fmt.Println(string(result))
	_, err = con.Write(result)
	_, err = con.Write([]byte("\n"))
	return err
}

func isPrime(number float64) bool {
	if math.Mod(number, 1.0) != 0 {return false}
	return big.NewInt(int64(number)).ProbablyPrime(0)
}