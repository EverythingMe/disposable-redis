// A utility to create disposable instances of redis server on random ports.
//
// This can be used for testing redis dependent code without having to make
// assumptions on if and where redis server is running, or fear of corrupting data.
// You create a redis server instance, run your code against it as if it were a mock,
// and then remove it without a trace.
package disposable_redis

import (
	"fmt"
	"log"
	"math/rand"
	"os/exec"
	"strings"
	"time"

	redigo "github.com/garyburd/redigo/redis"
)

// The redis executable. This allows you to set it if you're using a custom one.
// Can be an absolute path, or an executable in your $PATH
var RedisCommand = "redis-server"

const (
	MaxRetries = 10

	//this is the amount of time we give the server to start itself up and start listening (or fail)
	LaunchWaitTimeout = 100 * time.Millisecond
)

// A wrapper reperesenting a running disposable redis server
type Server struct {
	cmd     *exec.Cmd
	port    uint16
	running bool
}

// Start and run the process, return an error if it cannot be run
func (r *Server) run() error {

	ret := r.cmd.Start()

	ch := make(chan error)

	// we wait for LaunchWaitTimeout and see if the server quit due to an error
	go func() {
		err := r.cmd.Wait()
		select {
		case ch <- err:
		default:
		}
	}()

	select {
	case e := <-ch:
		log.Println("Error waiting for process:", e)
		return e
	case <-time.After(LaunchWaitTimeout):
		break

	}

	return ret
}

// Create and run a new server on a given port.
// Return an error if the server cannot be started
func NewServer(port uint16) (*Server, error) {

	cmd := exec.Command(RedisCommand,
		"--port", fmt.Sprintf("%d", port),
		"--pidfile", fmt.Sprintf("/tmp/disposable_redis.%d.pid", port),
		"--dir", "/tmp",
		"--dbfilename", fmt.Sprintf("dump.%d.%d.rdb", port, time.Now().UnixNano()),
	)

	log.Println("start args: ", cmd.Args)

	r := &Server{
		cmd:     cmd,
		port:    port,
		running: false,
	}

	err := r.run()
	if err != nil {
		return nil, err
	}
	r.running = true

	return r, nil

}

// Create a new server on a random port. If the port is taken we retry (10 times).
// If we still couldn't start the process, we return an error
func NewServerRandomPort() (*Server, error) {

	var err error
	var r *Server
	for i := 0; i < MaxRetries; i++ {
		port := uint16(rand.Int31n(0xffff-1025) + 1025)
		log.Println("Trying port ", port)

		r, err = NewServer(port)
		if err == nil {
			return r, nil
		}
	}

	log.Println("Could not start throwaway redis")
	return nil, err

}

// Wait for the server to be ready, or until a timeout has elapsed.
// This just blocks and waits using sleep intervals of 5ms if it can't connect
func (r *Server) WaitReady(timeout time.Duration) error {

	deadline := time.Now().Add(timeout)
	var err error

	for time.Now().Before(deadline) {

		conn, e := redigo.Dial("tcp", fmt.Sprintf("localhost:%d", r.port))
		if e != nil {
			log.Println("Could not connect, waiting 5ms")
			err = e
			time.Sleep(5 * time.Millisecond)
		} else {
			conn.Close()
			return nil
		}

	}
	return err

}

// Stop the running redis server
func (r *Server) Stop() error {
	if !r.running {
		return nil
	}
	r.running = false
	if err := r.cmd.Process.Kill(); err != nil {
		return err
	}

	r.cmd.Wait()

	return nil

}

// Info returns the value of the server's INFO command parsed into a map of strings
func (r Server) Info() (map[string]string, error) {
	conn, e := redigo.Dial("tcp", r.Addr())
	if e != nil {
		return nil, e
	}
	defer conn.Close()

	info, err := redigo.String(conn.Do("INFO"))
	if err != nil {
		return nil, err
	}

	ret := make(map[string]string)
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		kv := strings.Split(line, ":")

		if len(kv) == 2 {
			ret[kv[0]] = kv[1]
		}
	}

	return ret, nil
}

// NewSlaveOf creates a new server with a random port and makes it a slave of the current server.
func (r Server) NewSlaveOf() (*Server, error) {

	srv, err := NewServerRandomPort()
	if err != nil {
		return nil, err
	}

	if err = srv.WaitReady(100 * time.Millisecond); err != nil {
		defer srv.Stop()
		return nil, err
	}

	conn, e := redigo.Dial("tcp", srv.Addr())
	if e != nil {
		defer srv.Stop()
		return nil, e
	}
	defer conn.Close()

	_, err = conn.Do("SLAVEOF", "127.0.0.1", r.Port())
	if err != nil {
		defer srv.Stop()
		return nil, err
	}

	// wait for the slave to be in sync
	for i := 0; i < 100; i++ {
		info, err := srv.Info()
		if err != nil {
			defer srv.Stop()
			return nil, err
		}

		linkStatus := info["master_link_status"]
		if linkStatus == "up" {
			break
		}
		time.Sleep(50 * time.Millisecond)

	}

	return srv, nil

}

// Get the port of this server
func (r Server) Port() uint16 {
	return r.port
}

// Addr returns the address of the server as a host:port string
func (r Server) Addr() string {
	return fmt.Sprintf("localhost:%d", r.port)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
