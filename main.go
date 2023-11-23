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
	flag.IntVar(&challenge, "challenge",10, "Challenge number")
	flag.Parse()
	
	var port uint16 
  port =	13370
	var server Server
  switch challenge {
	  case 0:
	  	server = NewEchoServer(port);
		case 1:
	  	server = NewPrimeServer(port);
		case 2:
	  	server = NewMeansServer(port);
		case 3:
	  	server = NewChatServer(port);
		case 4:
	  	server = NewDatabaseServer(port);
		case 5:
	  	server = NewProxyServer(port);
		case 6:
	  	server = NewSpeedServer(port);
    case 7:
	  	server = NewReverseServer(port);
    case 8:
	  	server = NewSecureServer(port);
    case 9:
      server = NewJobServer(port);
    case 10:
      server = NewVcsServer(port);
		default:
			fmt.Printf("Unknown challenge\n")
			os.Exit(1)
		}
	
  server.Run()
}