# Discovery
```
$ nc vcs.protohackers.com 30307    
READY
help
OK usage: HELP|GET|PUT|LIST
READY                                      
list /                                     
OK 0                                       
READY                                      
put /snack 4                               
bla                                        
OK r1                                      
READY                                      
list /                                     
OK 1                                       
snack r1
READY
get /snack
OK 4
bla
READY
put /snack 3
al
OK r2
READY 
READY
list /
OK 1
snack r2
READY
put /snik/snak
ERR usage: PUT file length newline data
READY
put /snik/snak 3
ji
OK r1
READY
list /
OK 2
snack r2
snik/ DIR
READY
get
ERR usage: GET file [revision]
READY
get /snack r1
OK 4
bla
READY
```

# overview
this system is a simple filesystem that maintains file versions. All paths are absolute, there is no current working directory.

This is a tcp protocol that strictly prescribes who talks when. it has two modes:

* in command mode, server prints READY, then reads until it finds a newline
* in data mode, server reads a prespecified amount of bytes
* on connection, the server goes into command mode
* after a newline, if the command is put and it is a valid command, the server goes into data mode. after the specified amount of bytes has been read, the server prints a response (see below) and returns to command mode
* for all other commands, the server prints a response and returns to command mode
* responses start with OK or ERR and can be multiple lines
* if a non existing command is given, the response is 'ERR illegal method: [cmd]'

The filesystem must be global, shared by all users

Filenames are case sensitive

A node is listed as a file if it has 1 or more revisions. otherwise, it is listed as a dir. it can be both at the same time
# commands
commands and arguments are separated by spaces.

## help
OK response describing all commands. 
'OK usage: HELP|GET|PUT|LIST'

if arguments are given they are ignored

## LIST
argument: dir

if more or less arguments are provided, response is 'ERR usage: LIST dir'

dir must start with a / and can contain more than one / (subdirs). trailing / is optional. Two consecutive slashed are not allowed.

if dir is an illegal dir name (does not start with a /), response is 'ERR illegal dir name'

If dir does not exist or is empty, the response is 'OK 0'

the response followed by the number of lines in the list

each line starts with the name of the item

if the item is a directory, the name is postfixed with a / and followed by the word DIR

If the item is a file, the name is followed by the revision, prefixed by 'r'

## PUT
arguments: file length

if more or less arguments are provided, the response is 'ERR usage: PUT file length newline data'

filenames have the same rules as dirnames but can not end with a /

if an illegal filename is given, the response is 'ERR illegal file name'

if length is not a number or a negative number, it is interpreted as 0 

if no errors are found, the server goes into data mode to read the specified amount of bytes (can be 0)

the bytes are then stored in the file. if the file is new, it is given revision 1, otherwise the revision is incremented.

the response is 'OK r1' with r1 being the latest revision of the file.

## GET
arguments: file [revision]

If the wrong number of args is given, the response is 'ERR usage: GET file [revision]'

the file argument follows the same rules as PUT

if the file is not found, the response is 'ERR no such file'

if the file is a directory, the response is the same as when the file is not found

the revision is a number, optionally prefixed by a "r"

if the revision is postfixed by non-numeric chars, these are ignored

if the revision is not a number, it is not found

if the revision is not found, the response is 'ERR no such revision'

if no revision is given, the latest one is used.

The response is 'OK 5' where 5 is the number of bytes, followed by that number of bytes.

