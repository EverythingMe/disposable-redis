package disposable_redis

import (
	"fmt"
	"testing"
	"time"

	redigo "github.com/garyburd/redigo/redis"
)

// make sure we can't start 2 servers on the same port
func TestFailure(t *testing.T) {
	r, err := NewServerRandomPort()
	if err != nil {
		t.Fatal("Could not bind 1:", err)
	}
	defer r.Stop()

	if err != nil {
		t.Error("Could not connet", err)
	}

	r2, err2 := NewServer(r.Port())
	if err2 == nil {
		t.Error("We sohuldn't be able not create second instance")
		r2.Stop()
	}

}

func TestDisposableRedis(t *testing.T) {

	r, err := NewServerRandomPort()
	if err != nil {
		t.Fatal("Could not create random server")
	}

	defer r.Stop()

	if r.Port() < 1024 {
		t.Fatalf("Invalid port")
	}

	if err = r.WaitReady(50 * time.Millisecond); err != nil {
		t.Fatalf("Could not connect to server in time")
	}

	conn, err := redigo.Dial("tcp", fmt.Sprintf("localhost:%d", r.Port()))
	if err != nil {
		t.Fatalf("Could not connect to disposable server", err)
	}

	if _, err := conn.Do("PING"); err != nil {
		t.Fatalf("Could not talk to redis")
	}
	conn.Close()

	err = r.Stop()
	if err != nil {
		t.Fatal("Could not stop server", err)
	}

}

func TestServerNewSlaveOf(t *testing.T) {

	master, err := NewServerRandomPort()
	if err != nil {
		t.Fatal("Could not create random server")
	}

	defer master.Stop()

	if err = master.WaitReady(50 * time.Millisecond); err != nil {
		t.Fatalf("Could not connect to server in time")
	}

	slave, err := master.NewSlaveOf()
	if err != nil {
		t.Fatal("Could not create slave:", err)
	}
	defer slave.Stop()

	conn, err := redigo.Dial("tcp", fmt.Sprintf(master.Addr()))
	if err != nil {
		t.Fatalf("Could not connect to disposable server", err)
	}

	if _, err := conn.Do("SET", "foo", "bar"); err != nil {
		t.Fatalf("Could not talk to master")
	}
	conn.Close()

	time.Sleep(2000 * time.Millisecond)

	conn, err = redigo.Dial("tcp", fmt.Sprintf("localhost:%d", slave.Port()))
	if err != nil {
		t.Fatalf("Could not connect to slave server", err)
	}
	defer conn.Close()

	val, err := redigo.String(conn.Do("GET", "foo"))
	if err != nil {
		t.Fatal("Could not stop server", err)
	}

	if val != "bar" {
		t.Fatalf("Replication didn't work: ", val)
	}

}

func ExampleServer() {

	// create a new server on a random port
	r, err := NewServerRandomPort()
	if err != nil {
		panic("Could not create random server")
	}

	// we must remember to kill it at the end, or we'll have zombie redises
	defer r.Stop()

	// wait for our server to be ready for serving, for at least 50 ms.
	// This gives redis time to initialize itself and listen
	if err = r.WaitReady(50 * time.Millisecond); err != nil {
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
