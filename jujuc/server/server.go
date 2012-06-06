// The cmd/jujuc/server package allows a process to expose an RPC interface that
// allows client processes to delegate execution of cmd.Commands to a server
// process (with the exposed commands amenable to specialisation by context id).
package server

import (
	"bytes"
	"fmt"
	"launchpad.net/juju-core/juju/cmd"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"sync"
)

// Request contains the information necessary to run a Command remotely.
type Request struct {
	ContextId   string
	Dir         string
	CommandName string
	Args        []string
}

// Response contains the return code and output generated by a Request.
type Response struct {
	Code   int
	Stdout []byte
	Stderr []byte
}

// CmdGetter looks up a Command implementation connected to a particular Context.
type CmdGetter func(contextId, cmdName string) (cmd.Command, error)

// Jujuc implements the jujuc command in the form required by net/rpc.
type Jujuc struct {
	getCmd CmdGetter
}

// badReqErr returns an error indicating a bad Request.
func badReqErr(format string, v ...interface{}) error {
	return fmt.Errorf("bad request: "+format, v...)
}

// Main runs the Command specified by req, and fills in resp.
func (j *Jujuc) Main(req Request, resp *Response) error {
	if req.CommandName == "" {
		return badReqErr("command not specified")
	}
	if !filepath.IsAbs(req.Dir) {
		return badReqErr("Dir is not absolute")
	}
	c, err := j.getCmd(req.ContextId, req.CommandName)
	if err != nil {
		return badReqErr("%s", err)
	}
	var stdout, stderr bytes.Buffer
	ctx := &cmd.Context{req.Dir, &stdout, &stderr}
	resp.Code = cmd.Main(c, ctx, req.Args)
	resp.Stdout = stdout.Bytes()
	resp.Stderr = stderr.Bytes()
	return nil
}

// Server implements a server that serves command invocations via
// a unix domain socket.
type Server struct {
	socketPath string
	listener   net.Listener
	server     *rpc.Server
	closed     chan bool
	closing    chan bool
	wg         sync.WaitGroup
}

// NewServer creates an RPC server bound to socketPath, which can execute
// remote command invocations against an appropriate Context. It will not
// actually do so until Run is called.
func NewServer(getCmd CmdGetter, socketPath string) (*Server, error) {
	server := rpc.NewServer()
	if err := server.Register(&Jujuc{getCmd}); err != nil {
		return nil, err
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	s := &Server{
		socketPath: socketPath,
		listener:   listener,
		server:     server,
		closed:     make(chan bool),
		closing:    make(chan bool),
	}
	return s, nil
}

// Run accepts new connections until it encounters an error, or until Close is
// called, and then blocks until all existing connections have been closed.
func (s *Server) Run() (err error) {
	var conn net.Conn
	for {
		conn, err = s.listener.Accept()
		if err != nil {
			break
		}
		s.wg.Add(1)
		go func(conn net.Conn) {
			s.server.ServeConn(conn)
			s.wg.Done()
		}(conn)
	}
	select {
	case <-s.closing:
		// Someone has called Close(), so it is overwhelmingly likely that
		// the error from Accept is a direct result of the Listener being
		// closed, and can therefore be safely ignored.
		err = nil
	default:
	}
	s.wg.Wait()
	close(s.closed)
	return
}

// Close immediately stops accepting connections, and blocks until all existing
// connections have been closed.
func (s *Server) Close() {
	close(s.closing)
	s.listener.Close()
	os.Remove(s.socketPath)
	<-s.closed
}
