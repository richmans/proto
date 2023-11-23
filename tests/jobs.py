import socket
from struct import pack, unpack
from time import time, sleep
from binascii import unhexlify
import json

def sock():
  addr = ("localhost", 13370)
  s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
  s.connect(addr)
  return s

def snd(s, m):
  d = json.dumps(m)
  print("O", d)
  s.sendall(d.encode() +b'\n')

def rsp(s):
  d = s.recv(1024).decode().strip()
  print("I", d)
  j = json.loads(d)
  return j
  
def put(s, q="queue1", pri=123):
  j = {"request":"put","queue":"queue1","job":{"title": "blah"},"pri":123}
  snd(s, j)
  r = rsp(s)
  if "id" in r:
    return r["id"]
  return None

def get(s, q=["queue1"], wait=False):
  j = {"request":"get","queues":["queue1"], "wait": wait}
  snd(s, j)
  if wait:
    print("Waiting...")
    return None
  r = rsp(s)
  if "id" in r:
    return r["id"]
  return None
  
def abort(s, i):
  j = {"request":"abort","id": i}
  snd(s, j)
  rsp(s)

def delete(s, i):
  j = {"request":"delete","id": i}
  snd(s, j)
  rsp(s)
  
def bull1(s):
  snd(s, "snack")
  rsp(s)

def bull2(s):
  snd(s, {})
  rsp(s)

def bull3(s):
  snd(s,{"request":"snack"})
  rsp(s)

def test(s, s2):
  bull1(s)
  bull2(s)
  bull3(s)
  put(s) 
  jid = get(s)
  get(s)
  abort(s, jid)
  get(s)
  delete(s, jid)
  abort(s, jid)
  jid = put(s)
  get(s)
  get(s2)
  s.close()
  sleep(1)
  get(s2)
  delete(s2, jid)

def testw(s1, s2):
  get(s1, wait=True)
  put(s2)
  rsp(s1)
  
def testw2(s1, s2):
  put(s2)
  get(s2)
  get(s1, wait=True)
  sleep(1)
  s2.close()
  rsp(s1)
  
testw2(sock(),sock())