# Disposable-Redis
## Create disposable instances of redis server on random ports

This can be used for testing redis dependent code without having to make
assumptions on if and where redis server is running, or fear of corrupting data

For full documentation see [http://godoc.org/github.com/EverythingMe/disposable-redis](http://godoc.org/github.com/EverythingMe/disposable-redis)


## Example:

```go

import (
	"fmt"
	"time"
	disposable "github.com/EverythingMe/disposable-redis"
	redigo "github.com/garyburd/redigo/redis"
)

func ExampleServer() {

	// create a new server on a random port
	r, err := disposable.NewServerRandomPort()
	if err != nil {
		panic("Could not create random server")
	}

	// we must remember to kill it at the end, or we'll have zombie redises
	defer r.Stop()

	// wait for our server to be ready for serving, for at least 5 ms.
	// This gives redis time to initialize itself and listen
	if err = r.WaitReady(5 * time.Millisecond); err != nil {
		panic("Couldn't connect to instance")
	}

	//now we can just connect and talk to it
	conn, err := redigo.Dial("tcp", fmt.Sprintf("localhost:%d", r.Port()))
	if err != nil {
		panic(err)
	}

	fmt.Println(redigo.String(conn.Do("SET", "foo", "bar")))
	//Output: OK <nil>

}
```