package main

import (
"net"
"os"
"sort"
"fmt"
"time"
"encoding/binary"
"io"
"math"
"bytes"
)

type SpeedMessage interface {
	serialize() []byte
}

type ErrorMessage struct {
	err string
}

func serializeString(s string) []byte {
  return append([]byte{byte(len(s))}, []byte(s)...)
}

func (m ErrorMessage) serialize() []byte {
	return append([]byte{0x10}, serializeString(m.err)...)
}

type HeartbeatMessage struct {}

func (m HeartbeatMessage) serialize() []byte {
	return []byte{0x41}
}

type SpeedServer struct {
	port uint16
	tickQ chan bool
	delQ chan uint
  plateQ chan Observation
  dspQ chan uint
	clients map[uint]*SpeedClient
	numClients uint
  cars map[string]*Car
  pendingTickets map[uint16][]Ticket
}

func NewSpeedServer(port uint16) *SpeedServer {
	s := &SpeedServer{
		port,
		make(chan bool),
		make(chan uint),
    make(chan Observation),
    make(chan uint),
		make(map[uint]*SpeedClient),
		0,
    make(map[string]*Car),
    make(map[uint16][]Ticket),
	}
	
	return s
}

type SpeedClient struct {
	s *SpeedServer
	clientId uint
	con net.Conn
	q chan SpeedMessage
	ctype uint8
	lastHeartbeat int64
	heartbeat int64
	camRoad uint16
	camMile uint16
	camLimit uint16
  dspRoads []uint16
}

func NewSpeedClient(clientId uint, con net.Conn, s *SpeedServer) *SpeedClient {
	return &SpeedClient{
		s,
		clientId,
		con,
		make(chan SpeedMessage),
		0,
		0,
		0,
		0,
		0,
		0,
    nil,
	}
}

type Observation struct {
  plate string
  timestamp uint32
  road uint16
  mile uint16
  limit uint16
}

func (one Observation) speed(two Observation) uint16 {
  dist := math.Abs(float64(one.mile) - float64(two.mile))
  dur := math.Abs((float64(one.timestamp) - float64(two.timestamp)) / 3600)
  speed := (dist / dur) * 100
  //fmt.Printf("dist %f dur %f speed %f\n", dist, dur, speed)
  return uint16(speed)
}

type Ticket struct {
  plate string
  road uint16
  mile1 uint16
  timestamp1 uint32
  mile2 uint16
  timestamp2 uint32
  speed uint16 
}

func (t Ticket) serialize() []byte {
  buf := new(bytes.Buffer)
  buf.Write([]byte{0x21})
  binary.Write(buf, BE, uint8(len(t.plate)))
  buf.Write([]byte(t.plate))
  binary.Write(buf, BE, t.road)
  binary.Write(buf, BE, t.mile1)
  binary.Write(buf, BE, t.timestamp1)
  binary.Write(buf, BE, t.mile2)
  binary.Write(buf, BE, t.timestamp2)
  binary.Write(buf, BE, t.speed)
  return buf.Bytes()
}
type Car struct {
  plate string
  obs []Observation
  ticketDays map[uint32]bool
}

func (c *Car) hasTicket(one uint32, two uint32) bool {
  
  start := uint32(float64(one) / 86400)
  stop := uint32(float64(two) / 86400)
  for i := start; i <= stop; i+=1  {
    _, ok := c.ticketDays[i]
    if ok { 
      fmt.Printf("ticket on %d for %s\n", i, c.plate)
      return true
    }
    fmt.Printf("no ticket on %d for %s\n", i, c.plate)
  }
  
  return false
}

func (c *Car) markTicket(one uint32, two uint32) {
  start := uint32(float64(one) / 86400)
  stop := uint32(float64(two) / 86400)
  for i := start; i <= stop; i+=1  {
    fmt.Printf("setting ticket on %d for %s\n", i, c.plate)
    c.ticketDays[i] = true
  }
}

func (c *Car) addObservation(o Observation) int {
  i := sort.Search(len(c.obs), func(i int) bool { return c.obs[i].timestamp >= o.timestamp })
  c.obs = append(c.obs, o)
  copy(c.obs[i+1:], c.obs[i:])
  c.obs[i] = o
  return i
}

func (s *SpeedServer) tick() {
	cur := time.Now().UnixMilli()
	m := HeartbeatMessage{}
	for _, c := range s.clients {
		if c.heartbeat == 0 { continue }
	
		if c.lastHeartbeat + c.heartbeat < cur {
			c.q <- m
			c.lastHeartbeat = cur
		}
	}
}

func (s *SpeedServer) ticker() {
	for {
		s.tickQ <- true
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *SpeedServer) closeClient(cid uint) {
	c := s.clients[cid]
	delete(s.clients, cid)
	c.close()
}

func (s *SpeedServer) addPendingTicket(t Ticket) {
  l := s.pendingTickets[t.road]
  s.pendingTickets[t.road] = append(l, t)
}

func (s *SpeedServer) sendTicket(t Ticket) {
  for _, c := range s.clients {
    if c.handlesRoad(t.road) {
      c.q <- t
      return
    }
  }
  fmt.Printf("No dispatch available, pending ticket")
  s.addPendingTicket(t)
}

func (s *SpeedServer) flushDispatch(cid uint) {
  c, ok := s.clients[cid]
  if ! ok { return }
  fmt.Printf("flushing roads %v\n", c.dspRoads)
  for _, road := range c.dspRoads {
    fmt.Printf("finding pending tickets for %d\n", road)
    l, ok := s.pendingTickets[road]
    if ! ok { continue }
    fmt.Printf("ticket array %v\n", l)
    for _, t := range l {
      fmt.Printf("sending ticket")
      c.q <- t
    }
    delete(s.pendingTickets, road)
  }
}

func (s *SpeedServer) considerTicket(car *Car, pos int) {
  one := car.obs[pos]
  two := car.obs[pos + 1]
  if one.road != two.road { return }
  speed := one.speed(two)
  if speed <= one.limit { return }
  if car.hasTicket(one.timestamp, two.timestamp) { return }
  car.markTicket(one.timestamp, two.timestamp)
  fmt.Printf("Ticket %s, %d on road %d\n", car.plate, speed, one.road)
  t := Ticket {
    car.plate,
    one.road,
    one.mile,
    one.timestamp,
    two.mile,
    two.timestamp,
    speed,
  }
  s.sendTicket(t)
}

func (s *SpeedServer) plate(o Observation) {
  car, ok := s.cars[o.plate]
  if ! ok {
    car = &Car{
      o.plate, 
      nil,
      make(map[uint32]bool), 
    }
    s.cars[o.plate] = car
  }
  pos := car.addObservation(o)
  if pos > 0 {
    s.considerTicket(car, pos-1)
  }
  if pos < len(car.obs) - 1 {
    s.considerTicket(car, pos)
  }
}

func (s *SpeedServer)main() {
	for {
		select {
			case _ = <- s.tickQ: s.tick()
			case cid := <- s.delQ: s.closeClient(cid)
      case cid := <- s.dspQ: s.flushDispatch(cid)
      case o := <- s.plateQ: s.plate(o)
		}
	}
}

func (s *SpeedServer) Run() {
	go s.listen()
	go s.ticker()
	s.main()
}
	
func (s *SpeedServer) listen() {
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	server, err := net.Listen("tcp", addr)
    if err != nil {
      fmt.Println("Error listening:", err.Error())
        os.Exit(1)
    }
    defer server.Close()
    fmt.Println("SpeedServer waiting for client...")
    for {
      connection, err := server.Accept()
      if err != nil {
        fmt.Println("Error accepting: ", err.Error())
        os.Exit(1)
      }
      
      s.start(connection)
		}
}

func (s *SpeedServer) start(con net.Conn) {
	c := NewSpeedClient(s.numClients, con, s)
	s.clients[s.numClients] = c
	s.numClients += 1
	go c.sender()
	go c.receiver()
}

func (c *SpeedClient) sender() {
	for m := range c.q {
	  c.con.Write(m.serialize())
	}
	
	c.con.Close()
	fmt.Printf("Client %d closed\n", c.clientId)
}

func (c *SpeedClient) receiver() {
	var err error
	fmt.Printf("client %d connected\n", c.clientId)
	for {
		var mType uint8
		mType, err = readU8(c.con)
		if err != nil { break }
		switch mType {
      case 0x20: err = c.hdlPlate()
			case 0x40: err = c.hdlWantHeartbeat()
      case 0x80: err = c.hdlIAmCamera()
      case 0x81: err = c.hdlIAmDispatch()
			default: err = fmt.Errorf("Unknown message type 0x%x", mType)
		}
		if err != nil { break }
	}
	if err != nil {
    if err != io.EOF {
      m := ErrorMessage{err.Error()}
      c.q <- m
    }
		fmt.Printf("Client %d error %s\n", c.clientId, err)
	} else {
		fmt.Printf("Client %d closing\n", c.clientId)
	}
	c.s.delQ <- c.clientId
	
}

func (c *SpeedClient) close() {
	close(c.q)
}

func (c *SpeedClient) handlesRoad(road uint16) bool {
  if c.ctype != 2 { return false }
  for i := range c.dspRoads {
    if c.dspRoads[i] == road { return true}
  }
  return false
}
func (c *SpeedClient) hdlWantHeartbeat() error {
	wantedHb, err := readU32(c.con)
	if err != nil { return err }
	c.heartbeat = int64(wantedHb) * 100
	fmt.Printf("heartbeat for %d set to %d\n", c.clientId, wantedHb)
	return nil
}

func (c *SpeedClient) hdlIAmCamera() error {
  if c.ctype != 0 {
    return fmt.Errorf("wrong state")
  }
	c.ctype = 1
	var err error
	c.camRoad, err = readU16(c.con)
	if err != nil { return err }
	c.camMile, err = readU16(c.con)
	if err != nil { return err }
	c.camLimit, err = readU16(c.con)
  c.camLimit = c.camLimit * 100
	fmt.Printf("Client %d is camera %d/%d limit %d\n", c.clientId, c.camRoad, c.camMile, c.camLimit)
	return err
}

func (c *SpeedClient) hdlIAmDispatch() error {
  if c.ctype != 0 {
    return fmt.Errorf("wrong state")
  }
  c.ctype = 2
  numRoads, err := readU8(c.con)
  if err != nil { return err }
  for i := uint8(0); i < numRoads; i++ {
    road, err := readU16(c.con)
    if err != nil { return err }
    c.dspRoads = append(c.dspRoads, road)
  }
  c.s.dspQ <- c.clientId
  fmt.Printf("Client %d is dispatch for %v\n", c.clientId, c.dspRoads)
  return nil
}

func (c *SpeedClient) hdlPlate() error {
  if c.ctype != 1 {
    return fmt.Errorf("not a camera")
  }
  plate, err := readString(c.con)
  if err != nil { return err }
  timestamp, err := readU32(c.con)
  if err != nil { return err }
  o := Observation{
    plate,
    timestamp,
    c.camRoad,
    c.camMile,
    c.camLimit,
  }
  fmt.Printf("Cam %d saw %s on %d/%d at %d\n", c.clientId, plate, c.camRoad, c.camMile, timestamp)
  c.s.plateQ <- o
  return nil
}