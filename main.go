package main

import (
  "fmt"
	"flag"
	"os"
)

type Server interface {
	Run()
}
	
func main() {
	var challenge int
	flag.IntVar(&challenge, "challenge",1, "Challenge number")
	flag.Parse()
	
	var port uint16 
  port =	13370
	var server Server
  switch challenge {
	  case 0:
	  	server = NewEchoServer(port);
		case 1:
	  	server = NewPrimeServer(port);
		default:
			fmt.Printf("Unknown challenge\n")
			os.Exit(1)
		}
	
  server.Run()
}