package main

import (
"net"
"os"
"fmt"
"bufio"
"encoding/json"
"sort"
)

func InsertJobSorted(s []QueuedJob, e QueuedJob) []QueuedJob {
	
	i := sort.Search(len(s), func(i int) bool { return s[i].Pri > e.Pri })
  s = append(s, e)
	copy(s[i+1:], s[i:])
	s[i] = e
	return s
}   

type ErrorResponse struct{
  Status string `json:"status"`
  Error string `json:"error"`
}

type PutResponse struct {
  Status string `json:"status"`
  Id uint `json:"id"`
}

type GetResponse struct {
  Status string `json:"status"`
  Id uint `json:"id"`
  Job interface{} `json:"job"`
  Pri uint `json:"pri"`
  Queue string `json:"queue"`
}

type GetErrorResponse struct {
  Status string `json:"status"`
}

type JobRequest struct {
  From uint
	Request string
  Queue string
  Pri uint
	Job interface{}
  Queues []string
  Wait bool
  Id uint
}

type Job struct {
  Id uint
  Details interface{}
  Pri uint
  Worker uint
  Queue string
}

type QueuedJob struct {
  Id uint
  Pri uint
}

type JobServer struct {
	port uint16
	numSessions uint
  numJobs uint
	q chan JobRequest
	clients map[uint]*JobSession
  queues map[string][]QueuedJob
  waiters map[string][]uint
  jobs map[uint]*Job
}

func NewJobServer(port uint16) *JobServer {
	return &JobServer{
		port,
		0,
    0,
		make(chan JobRequest),
		make(map[uint]*JobSession),
    make(map[string][]QueuedJob),
    make(map[string][]uint),
    make(map[uint]*Job),
	}
}

type JobSession struct{
	sessionId uint
	q chan []byte
	backend chan JobRequest
  jobs map[uint]bool
	con net.Conn
	isOnline bool
}

func NewJobSession(sessionId uint, con net.Conn, backend chan JobRequest) *JobSession {
	return &JobSession{
		sessionId,
		make(chan []byte),
		backend,
    make(map[uint]bool),
		con,
		true,
	}
}

func (s *JobServer) ensureQueue(q string) {
  _, ok := s.queues[q]
  if ! ok { 
    s.queues[q] = make([]QueuedJob, 0, 5)
  }
  _, ok = s.waiters[q]
  if ! ok {
    s.waiters[q] = make([]uint, 0, 5)
  }
}

func (s *JobServer) pushWaiter(q string, j *Job) bool {
  
  for len(s.waiters[q]) > 0 {
    sid := s.waiters[q][0]
    s.waiters[q] = s.waiters[q][1:]
    ses, ok := s.clients[sid]
    if ! ok { continue }
    if ! ses.isOnline { continue }
    j.Worker = sid
    s.rspGet(ses, j)
    return true
  }
  return false
  
}
func (s *JobServer) pushJob(j *Job, q string) {
  s.ensureQueue(q)
  if s.pushWaiter(q, j) { return }
  qj := QueuedJob{j.Id, j.Pri}
  s.queues[q] = InsertJobSorted(s.queues[q], qj)
}

func (s *JobServer) rspPut(ses *JobSession, id uint) {
  m := PutResponse{"ok", id}
  d, _ := json.Marshal(m)
  d = append(d, '\n')
  ses.q <- d
}

func (s *JobServer) hdlPut(r JobRequest, ses *JobSession) {
  s.numJobs += 1
  j := &Job{s.numJobs, r.Job, r.Pri, 0, r.Queue}
  s.jobs[j.Id] = j
  s.pushJob(j, r.Queue)
  s.rspPut(ses, j.Id)
}

func (s *JobServer) getBestQueue(qs []string) (string, bool) {
  var bestPri uint
  var bestQ string
  found := false
  for qi := range qs {
    qname := qs[qi]
    q, ok := s.queues[qname]
    if ! ok { continue }
    if len(q) == 0 { continue }
    j := q[len(q) -1]
    if found == false || j.Pri >= bestPri {
      bestPri = j.Pri
      bestQ = qname
    }
    found = true  
  }
  return bestQ, found
}

func (s *JobServer) popQueue(qname string) (*Job, error) {
  q, ok := s.queues[qname]
  if ! ok { return  nil, fmt.Errorf(  "queue %s not found", qname)}
  if len(q)== 0 { return nil, fmt.Errorf("queue %s empty", qname)}
  it := q[len(q)-1]
  s.queues[qname] = q[:len(q)-1]
  j, ok := s.jobs[it.Id]
  if ok {
    return j, nil
  } else {
    return nil, fmt.Errorf("job not found")
  }
}

func (s *JobServer) rspError(ses *JobSession, err string) {
  m := ErrorResponse{"error", err}
  d, _ := json.Marshal(m)
  d = append(d, '\n')
  ses.q <- d
}

func (s *JobServer) rspGet(ses *JobSession, j *Job) {
  m := GetResponse{"ok", j.Id, j.Details, j.Pri, j.Queue}
  d, _ := json.Marshal(m)
  ses.jobs[j.Id] = true
  d = append(d, '\n')
  ses.q <- d
}

func (s *JobServer) rspGetNoJob(ses *JobSession) {
  m := GetErrorResponse{"no-job"}
  d, _ := json.Marshal(m)
  d = append(d, '\n')
  ses.q <- d
}

func (s *JobServer) rspOk(ses *JobSession) {
  m := GetErrorResponse{"ok"}
  d, _ := json.Marshal(m)
  d = append(d, '\n')
  ses.q <- d
}

func (s *JobServer) waitFor(ses *JobSession, qs []string) {
  for _, qname := range qs {
    s.ensureQueue(qname)
    s.waiters[qname] = append(s.waiters[qname], ses.sessionId)
  }
  
}

func (s *JobServer) hdlGet(r JobRequest, ses *JobSession) {
  for {
    q, ok := s.getBestQueue(r.Queues)
    if ! ok {
      if r.Wait {
        s.waitFor(ses, r.Queues)
      } else {
        s.rspGetNoJob(ses)
      }
      return
    }
    j, err := s.popQueue(q)
    if err != nil {
      continue
    }
    s.rspGet(ses, j)
    j.Worker = ses.sessionId
    return
  }
  
}

func (s *JobServer) hdlAbort(r JobRequest, ses *JobSession) {
  j, ok := s.jobs[r.Id]
  if ! ok {
    s.rspGetNoJob(ses)
    return
  }
  if j.Worker != ses.sessionId {
    s.rspError(ses, "not your job")
    return
  }
  if _, ok := ses.jobs[r.Id]; ok {
    delete(ses.jobs, r.Id)
  }
  j.Worker = 0
  s.pushJob(j, j.Queue)
  s.rspOk(ses)
}

func (s *JobServer) hdlDelete(r JobRequest, ses *JobSession) {
  _, ok := s.jobs[r.Id]
  if ! ok {
    s.rspGetNoJob(ses)
    return
  }
  delete(s.jobs, r.Id)
  s.rspOk(ses)
}

func (s *JobServer) hdlClose(ses *JobSession) {
  for jid, _ := range ses.jobs {
    j, ok := s.jobs[jid]
    if ! ok { continue }
    j.Worker = 0
    s.pushJob(j, j.Queue)
  }
}

func (s *JobServer) central(){
	for m := range s.q {
    ses, ok := s.clients[m.From]
    if ! ok { continue }
		//fmt.Printf("I %+v\n", m)
    switch m.Request {
      case "put": s.hdlPut(m, ses)
      case "get": s.hdlGet(m, ses)
      case "abort": s.hdlAbort(m, ses)
      case "delete": s.hdlDelete(m, ses)
      case "close": s.hdlClose(ses)
      default: s.rspError(ses, "unknown request type")
    }
	}
}


func (s *JobServer) Run() {
	go s.central()
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	server, err := net.Listen("tcp", addr)
    if err != nil {
      fmt.Println("Error listening:", err.Error())
        os.Exit(1)
    }
    defer server.Close()
    fmt.Println("JobServer waiting for client...")
    for {
      connection, err := server.Accept()
      if err != nil {
        fmt.Println("Error accepting: ", err.Error())
        os.Exit(1)
      }
      fmt.Println("client connected")
      s.processClient(connection)
		}
		close(s.q)
}

func (s *JobServer) processClient(con net.Conn) {
	sessionId := s.numSessions
	s.numSessions += 1
	session := NewJobSession( sessionId, con, s.q)
	s.clients[sessionId] = session
	go session.jobReceiver()
	go session.jobSender()
}

func (s *JobSession) close() {
	//fmt.Printf("close %d\n", s.sessionId)
	if s.isOnline {
    s.isOnline = false
    m := JobRequest{
      s.sessionId,
	    "close",
      "",
      0,
      nil,
      []string{},
      false,
      0,
    }
	  s.backend <- m
	}
  s.con.Close()
}

func ValidatePut(r JobRequest) error {
  if r.Job == nil {
    return fmt.Errorf("missing job field")
  }
  if r.Queue == "" {
    return fmt.Errorf("missing queue field")
  }
  return nil
}

func ValidateRequest(r JobRequest) error {
  switch r.Request {
    case "put": return ValidatePut(r)
    
  }
  return nil
}
func ParseRequest(msg string) (JobRequest, error) {
  var r JobRequest
  err := json.Unmarshal([]byte(msg), &r)
  if err != nil { return r, err}
  //fmt.Printf("%+v\n", r)
  return r, nil
}

func errorMessage(err error) []byte {
  m := ErrorResponse{"error", err.Error()}
  s, _ := json.Marshal(m)
  s = append(s, '\n')
  return s
}

func (s *JobSession) jobReceiver() {
	defer s.close()
	r := bufio.NewReaderSize(s.con, 102400)
	msg, err := r.ReadString('\n')
  
	for err == nil {
    fmt.Printf("%s", msg)
	  rq, perr := ParseRequest(msg)
    if perr == nil {
      perr = ValidateRequest(rq)
    }
    if perr == nil {
      rq.From = s.sessionId
  	  s.backend <- rq
    } else {
      s.q <- errorMessage(perr)
    }
    msg, err = r.ReadString('\n')
	}
}

func (s *JobSession) jobSender() {
	for m := range s.q {
		s.con.Write(m)
	}
}

