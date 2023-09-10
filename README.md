# gRPC pub(sub) quiz

### Building
#### Generate gRPC code
```bash
$ make generate_grpc
```

#### Compiling
To build for desired platform, run either of the following command:
```bash
$ make build_linuxAmd64
$ make build_darwinAmd64
$ make build_windowsAmd64
```

#### Running server
```bash
$ ./bin/server --help                                                 
Usage of ./bin/server:
  -port int
    	port number (default 50052)
  -questionFile string
    	Path to the question file (default "./questions_short.json")
  -questionTimeout duration
    	Timeout for each question (default 10s)
  -startDelay duration
    	Delay before starting the quiz (default 10s)
  -targetPlayers int
    	Number of players (default 1)
```

#### Running the client
```bash
$ ./bin/client --help
Usage of ./bin/client:
  -addr string
    	the address to connect to (default "localhost:50052")
```

###### References
1. Questions: https://github.com/ThwCorbin/pub-quiz


## License
[LICENSE](github.com/Lastin/pub-sub-quiz/blob/master/LICENSE.txt)