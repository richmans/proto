import sys

ses = {}

fil = open(sys.argv[1])
for l in fil:
  l = l.strip()
  p = l.split("]")
  tst = p[1][2:]
  txt = p[2].strip()
  tp = txt.split(" ")
  print(txt)
  if txt.startswith("NOTE:successfully connected with session"):
    sid = int(tp[-1])
    ses[sid] = [sid, tst, False]
  if txt.startswith("NOTE:closed session"):
    sid = int(tp[2])
    ses[sid][2] = True
print("unclosed sessions:")
for v in ses.values():
  if v[2] == False:
    print(v[1], v[0])
    
  