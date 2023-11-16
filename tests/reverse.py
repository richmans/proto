import socket
from time import time

addr = ("localhost", 13370)
ses = int(time())
def sock():
  s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
  return s

def connect(s):
  s.sendto(f"/connect/{ses}/".encode(), addr)

def data(s):
  s.sendto(f"/data/{ses}/0/12/".encode(), addr)
def data2(s):
  s.sendto(f"/data/{ses}/2/Hello world this is a long long test!\n/".encode(), addr)

def escdata(s):
   s.sendto(f"/data/{ses}/0/foo\/bar\/baz\nfoo\\bar\\baz\n/".encode(), addr)

def chunk1(s):
  s.sendto(f"/data/{ses}/0/snack/".encode(), addr)

def chunk2(s):
  s.sendto(f"/data/{ses}/0/snack snoep\nsnap/".encode(), addr)

def chunk3(s):
  s.sendto(f"/data/{ses}/0/snack snoep\nsnap snep\nslap\n/".encode(), addr)

def simple(s):
  connect(s)
  print(s.recv(1024).decode())
  data2(s)
  print(s.recv(1024).decode())
  data2(s)
  print(s.recv(1024).decode())
  data(s)
  print(s.recv(1024).decode())
  print(s.recv(1024).decode())
  print(s.recv(1024).decode())

def escaper(s):
  connect(s)
  print(s.recv(1024).decode())
  escdata(s)
  
  print(s.recv(1024).decode())

def waiter(s):
  connect(s)
  print(s.recv(1024).decode())
  
  
def chunker(s):
  connect(s)
  print(s.recv(1024).decode())
  chunk1(s)
  print(s.recv(1024).decode())
  chunk2(s)
  print(s.recv(1024).decode())
  chunk3(s)
  print(s.recv(1024).decode())
  print(s.recv(1024).decode())
  
  
#simple(sock())
#escaper(sock())
#waiter(sock())
chunker(sock())