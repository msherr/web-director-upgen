# web-director

## Compiling

```bash
go build ./cmd/server
go build ./cmd/client
```

Note that the client is for testing purposes.

## Running the server:

You need to set the `SERVER_AUTH_TOKEN` environment variable.  Share this securely with your client(s).

You also need to pass in the certificate and the certificate key.  

Some example invocations:
```bash
./server -h
```
To cause the server to actually start, do something like:
```bash
SERVER_AUTH_TOKEN=micah1 ./server -certpath mytest.pem -keypath mytest-key.pem
```


## API

### /version

Returns the current version number

### /exit

Causes the server to quit

### /runToCompletion

Runs a program to completion.  Requires the following argument:

```go
// jsonCommandStruct represents a command to be executed.
type jsonCommandStruct struct {
	TimeoutInSecs float32  `json:"timeout"` // anything positive will be considered a timeout
	Cmd           string   `json:"cmd"`
	Args          []string `json:"args"`
}
```


### /runInBackground

Runs a program in the background.  Requires the following argument (same as above):

```go
// jsonCommandStruct represents a command to be executed.
type jsonCommandStruct struct {
	TimeoutInSecs float32  `json:"timeout"` // anything >0 will be considered a timeout
	Cmd           string   `json:"cmd"`
	Args          []string `json:"args"`
}
```

### /jobs

Lists the running jobs

### /kill

Terminates a job.  Requires the following argument:

```go
// jsonKillStruct represents a job to be killed.
// note that  if job == -1, then all jobs will be killed
type jsonKillStruct struct {
	JobNo JobNoType `json:"job"`
}
```


## Testing

```
curl --header "Content-Type: application/json" \
  -H "X-Session-Token: micah1" \
  --request POST \
  --data '{"username":"xyz","password":"xyz"}' \
  https://localhost:8888/exit
```