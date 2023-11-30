package main

import (
"net"
"os"
"fmt"
"bufio"
"bytes"
)

const PACKET_HELLO = 0x50
const PACKET_ERROR = 0x51
const PACKET_SITE_VISIT = 0x58
const PACKET_DIAL_AUTHORITY = 0x53
const PACKET_TARGET_POPULATIONS = 0x54
const PACKET_CREATE_POLICY = 0x55
const PACKET_DELETE_POLICY = 0x56
const PACKET_DELETE_POLICY_OK = 0x52
const PACKET_CREATE_POLICY_OK = 0x57

type PacketReader struct {
  count uint
  sum byte
  ptype byte
  plen uint
  base *bufio.Reader
  started bool
}
func newPacketReader(r *bufio.Reader) *PacketReader {
  return &PacketReader{
    0,
    0,
    0,
    0,
    r,
    false,
  }
}

func (r *PacketReader) Read(b []byte) (int, error) {
  bytesLeft := r.plen - r.count
  if len(b) > int(bytesLeft) {
    return 0, fmt.Errorf("not enough bytes left in package")
  }
  if ! r.started {
    return 0, fmt.Errorf("read called but not started")
  }
  n, err := r.base.Read(b)
  if err != nil { return 0, err }
  for i := 0; i < n; i++ {
    r.sum += b[i]
  }
  r.count += uint(len(b))
  return n, err
}

func (r *PacketReader) readError() error {
  plen, err := readU32(r) //plen
  r.plen = uint(plen)
  if err != nil { return err }
  d, err := readLString(r)
  _, err = readU8(r) // checksum
  r.started = false
  if err != nil { return err }
  return fmt.Errorf("Server error: %s", d)
}

func (r *PacketReader) start(ptype byte) error {
  if r.started {
    return fmt.Errorf("already started")
  }
  var err error
  r.plen = 5
  r.sum = 0
  r.count = 0
  r.started = true
  r.ptype, _ = readU8(r)
  if r.ptype == PACKET_ERROR {
    err = r.readError()
    return err
  }
  
  if r.ptype != ptype {
    return fmt.Errorf("expected packet type %x but found %x", ptype, r.ptype)
  }
  plen, _ := readU32(r)
  r.plen = uint(plen)
  return nil
}

func (r *PacketReader) finish() error {
  if ! r.started {
    return fmt.Errorf("not started")
  }
  _, err := readU8(r) //checksum
  r.started = false
  if err != nil { return err }
  if r.count != r.plen {
    return fmt.Errorf("packet len mismatch exp %d read %d", r.plen, r.count)
  }
  if r.sum != 0 {
    return fmt.Errorf("checksum error: %d", r.sum)
  }
  
  return nil
}

type PacketWriter struct {
  count uint
  sum byte
  ptype byte
  buf *bytes.Buffer
  base *bufio.Writer
  started bool
}

func newPacketWriter(r *bufio.Writer) *PacketWriter {
  return &PacketWriter{
    0,
    0,
    0,
    nil,
    r,
    false,
  }
}

func (w *PacketWriter) start(ptype byte) error {
  if w.started {
    return fmt.Errorf("already started")
  }
  w.buf = bytes.NewBuffer([]byte{})
  w.count = 0
  w.sum = ptype
  w.started = true
  w.ptype = ptype
  return nil
}

func (w *PacketWriter) finish() error {
  if ! w.started {
    return fmt.Errorf("not started")
  }
  w.started = false
  plen := uint32(w.count + 6)
  c := byte(plen) + byte(plen >> 8) + byte(plen >> 16) + byte(plen >> 24) + w.sum
  c = byte(- c)
  err := writeU8(w.base, w.ptype)
  if err != nil { return err }
  err = writeU32(w.base, plen)
  if err != nil { return err }
  _,err = w.base.Write(w.buf.Bytes())
  if err != nil { return err }
  err = writeU8(w.base, c)
  if err != nil { return err }
  //fmt.Printf("Sending packet %x length %d, data %d\n", w.ptype, plen, w.buf.Len())
  err = w.base.Flush()
  return err
}

func (w *PacketWriter) Write(b []byte) (int, error) {
  if ! w.started {
    return 0, fmt.Errorf("write called but not started")
  }
  n, err := w.buf.Write(b)
  w.count += uint(n)
  for i := 0; i < n; i++ {
    w.sum += b[i]
  }
  return n, err
}


type HelloPacket struct {
  protocol string
  version uint
}

func readHelloPacket(r *PacketReader) (HelloPacket, error) {
  var p HelloPacket
  var err error
  err = r.start(PACKET_HELLO)
  if err != nil { return p, err }
  p.protocol, err = readLString(r)
  if err != nil { return p, err }
  version, err := readU32(r)
  if err != nil { return p, err }
  p.version = uint(version)
  err = r.finish()
  return p, err
}

func writeHelloPacket(w *PacketWriter, p HelloPacket) error {
  err := w.start(PACKET_HELLO)
  if err != nil { return err }
  err = writeLString(w, p.protocol)
  if err != nil { return err }
  err = writeU32(w, uint32(p.version))
  if err != nil { return err }
  err = w.finish()
  return err
}

func readHello(r *PacketReader) error {
  hello, err := readHelloPacket(r)  
  if err != nil { return err }
  fmt.Printf("%+v\n", hello)
  if hello.protocol != "pestcontrol" { return fmt.Errorf("unknown protocol")}
  if hello.version != 1 {
    return fmt.Errorf("unknown version")
  }
  return nil
}

func sendHello(w *PacketWriter) error {
  p := HelloPacket{"pestcontrol", 1}
  err := writeHelloPacket(w, p)
  return err
}

type Tally struct {
  site uint
  species string
  count uint
}

type SiteVisitPacket struct {
  siteId uint
  populations []Tally
}

func validateSiteVisit(v SiteVisitPacket) error {
  s := make(map[string]uint)
  for _, t := range v.populations {
    t2, ok := s[t.species]
    if ok && t.count != t2 {
      return fmt.Errorf("conflicting counts for %s", t.species)
    }
    s[t.species] = t.count
  }
  return nil
}

func readSiteVisitPacket(r *PacketReader) (SiteVisitPacket, error) {
  var p SiteVisitPacket
  var err error
  err = r.start(PACKET_SITE_VISIT)
  if err != nil { return p, err }
  siteId, err := readU32(r)
  if err != nil { return p, err }
  p.siteId = uint(siteId)
  talCount, err := readU32(r)
  if err != nil { return p, err }
  for i := 0 ; i < int(talCount) ; i++ {
    var t Tally
    var count uint32
    t.site = uint(siteId)
    t.species, err = readLString(r)
    if err != nil { return p, err }
    count, err = readU32(r)
    if err != nil { return p, err }
    t.count = uint(count)
    p.populations = append(p.populations, t)
  }
  err = r.finish()
  return p, err
}

type DialAuthorityPacket struct {
  siteId uint
}

func writeDialAuthorityPacket(w *PacketWriter, p DialAuthorityPacket) error {
  err := w.start(PACKET_DIAL_AUTHORITY)
  if err != nil { return err }
  err = writeU32(w, uint32(p.siteId))
  if err != nil { return err }
  err = w.finish()
  return err
}

type MaxPop struct {
  species string
  max uint
  min uint
  
}

type TargetPopulationsPacket struct {
  siteId uint
  populations []MaxPop
}

func readTargetPopulationsPacket(r *PacketReader) (TargetPopulationsPacket, error) {
  var p TargetPopulationsPacket
  var err error
  err = r.start(PACKET_TARGET_POPULATIONS)
  if err != nil { return p, err }
  siteId, err := readU32(r)
  if err != nil { return p, err }
  p.siteId = uint(siteId)
  talCount, err := readU32(r)
  if err != nil { return p, err }
  for i := 0 ; i < int(talCount) ; i++ {
    var t MaxPop
    var max uint32
    var min uint32
    t.species, err = readLString(r)
    if err != nil { return p, err }
    max, err = readU32(r)
    if err != nil { return p, err }
    t.max = uint(max)
    min, err = readU32(r)
    if err != nil { return p, err }
    t.min = uint(min)
    p.populations = append(p.populations, t)
  }
  err = r.finish()
  return p, err
}

type CreatePolicyPacket struct {
  species string
  policy byte
}

func writeCreatePolicyPacket(w *PacketWriter, p CreatePolicyPacket) error {
  err := w.start(PACKET_CREATE_POLICY)
  if err != nil { return err }
  err = writeLString(w, p.species)
  if err != nil { return err }
  err = writeU8(w, p.policy)
  if err != nil { return err }
  err = w.finish()
  return err
}

type DeletePolicyPacket struct {
  policyId uint
}

func writeDeletePolicyPacket(w *PacketWriter, p DeletePolicyPacket) error {
  err := w.start(PACKET_DELETE_POLICY)
  if err != nil { return err }
  err = writeU32(w, uint32(p.policyId))
  if err != nil { return err }
  err = w.finish()
  return err
}

type CreatePolicyOkPacket struct {
  policyId uint
}
func readCreatePolicyOkPacket(r *PacketReader) (CreatePolicyOkPacket, error) {
  var err error
  var p CreatePolicyOkPacket
  err = r.start(PACKET_CREATE_POLICY_OK)
  if err != nil { return p, err }
  pid, err := readU32(r)
  if err != nil { return p, err }
  p.policyId = uint(pid)
  err = r.finish()
  return p, err
}

func readDeletePolicyOkPacket(r *PacketReader) error {
  var err error
  err = r.start(PACKET_DELETE_POLICY_OK)
  if err != nil { return err }
  err = r.finish()
  return err
}

func writeError(w *PacketWriter, e error) error {
  err := w.start(PACKET_ERROR)
  if err != nil { return err }
  err = writeLString(w, e.Error())
  if err != nil { return err }
  err = w.finish()
  return err
}

type Policy struct {
  policyId uint
  policy byte
}

type Authority struct {
  siteId uint
  q chan SiteVisitPacket
  r *PacketReader
  w *PacketWriter
  pops map[string]MaxPop
  policies map[string]Policy
  
}

func newAuthority(siteId uint) (*Authority) {
  a := &Authority{
    siteId,
    make(chan SiteVisitPacket, 10),
    nil,
    nil,
    make(map[string]MaxPop),
    make(map[string]Policy),
  }
  go a.run()
  return a
}

func (a *Authority) connect() error {
  addr := "pestcontrol.protohackers.com:20547"
	con, err := net.Dial("tcp", addr)
	if err != nil {	return err }
  a.r = newPacketReader(bufio.NewReader(con))
  a.w = newPacketWriter(bufio.NewWriter(con))
  sendHello(a.w)
  err = readHello(a.r)
  if err != nil { return err}
  p := DialAuthorityPacket{a.siteId}
  err = writeDialAuthorityPacket(a.w, p)
  if err != nil { return err}
  mp, err := readTargetPopulationsPacket(a.r)
  if err != nil { return err}
  for _, p := range mp.populations {
    a.pops[p.species] = p
  }
  return nil
}

func getMeasuredCount(v SiteVisitPacket, species string) uint {
  for _, p := range v.populations {
    if p.species == species {
      return p.count
    }
  }
  return 0
}

func (a *Authority) deletePolicy(policyId uint) error {
  p := DeletePolicyPacket{policyId}
  err := writeDeletePolicyPacket(a.w, p)
  if err != nil { return err }
  err = readDeletePolicyOkPacket(a.r)
  fmt.Printf("delpol %d\n", policyId)
  return err
}

func (a *Authority) createPolicy(species string, policy byte) (uint, error) {
  p := CreatePolicyPacket{species, policy}
  err := writeCreatePolicyPacket(a.w, p)
  if err != nil { return 0, err }
  okp, err := readCreatePolicyOkPacket(a.r)
  fmt.Printf("pol %s set to %d id %d\n", species, policy, okp.policyId)
  return okp.policyId, err
}

func (a *Authority) setPolicy(species string, policy byte) error {
  var err error
  pol, ok := a.policies[species]
  // if the policy matches, done!
  if pol.policy == policy {
    return nil
  }
  if ok {
    err = a.deletePolicy(pol.policyId)
  }
  if err != nil { return err }
  
  pol.policy = policy
  pol.policyId, err = a.createPolicy(species, policy)
  a.policies[species] = pol
  return err
}

func(a *Authority) hdlSiteVisit(v SiteVisitPacket) error  {
  for _, p := range a.pops {
    cnt := getMeasuredCount(v,  p.species)
    policy := byte(0)
    if cnt > p.max { policy = 0x90 }
    if cnt < p.min { policy = 0xa0 }
    err := a.setPolicy(p.species, policy)
    if err != nil { return err }
  }
  return nil
}

func (a *Authority) run() {
  fmt.Printf("connecting to site %d\n", a.siteId)
  err := a.connect()
  if err != nil {
    fmt.Printf("ERROR authority connect fail %s\n", err)
    return // todo retry later
  } 
  for v := range a.q {
    err = a.hdlSiteVisit(v)
    if err != nil {
      fmt.Printf("ERROR %s\n", err)
    }
  }
}

type PestServer struct {
	port uint16
  q chan SiteVisitPacket
  authorities map[uint]*Authority
}

func NewPestServer(port uint16) *PestServer {
	return &PestServer{
		port,
    make(chan SiteVisitPacket, 10),
    make(map[uint]*Authority),
	}
}

type PestSession struct{
	con net.Conn
  backend chan SiteVisitPacket
  r *PacketReader
  w *PacketWriter
}

func NewPestSession(con net.Conn, backend chan SiteVisitPacket) *PestSession {
	return &PestSession{
    con,
    backend,
    newPacketReader(bufio.NewReader(con)),
    newPacketWriter(bufio.NewWriter(con)),
	}
}

func (s *PestServer) central() {
  for t := range s.q {
    a, ok := s.authorities[t.siteId]
    if ! ok {
      a = newAuthority(t.siteId)
      s.authorities[t.siteId] = a
    }
    a.q <- t
  }
}

func (s *PestServer) Run() {
  go s.central()
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	server, err := net.Listen("tcp", addr)
    if err != nil {
      fmt.Println("Error listening:", err.Error())
        os.Exit(1)
    }
    defer server.Close()
    fmt.Println("PestServer waiting for client...")
    for {
      connection, err := server.Accept()
      if err != nil {
        fmt.Println("Error accepting: ", err.Error())
        os.Exit(1)
      }
      fmt.Println("client connected")
      s.processClient(connection)
		}
}

func (s *PestServer) processClient(con net.Conn) {
	session := NewPestSession(con, s.q)
	go session.pestHandler()
}


func (s *PestSession) pestHandler() error {
	err := readHello(s.r)
  if err != nil { return err }
  fmt.Println("got hello, sending back")
  err = sendHello(s.w)
  var v SiteVisitPacket
  for err == nil {
    v, err = readSiteVisitPacket(s.r)
    if err != nil { return err }
    fmt.Printf("I %+v\n", v)
    err := validateSiteVisit(v)
    if err != nil {
      writeError(s.w, err)
    } else {
      s.backend <- v
    }
  }
  return err
}

func (s *PestSession) run() {
  err := s.pestHandler()
  fmt.Printf("%e\n", err)
  if err != nil {
    writeError(s.w, err)
  }
}
