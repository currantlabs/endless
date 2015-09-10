package endless

import (
	"log"
	"net"
	"os"
	"syscall"
	"time"
	"sync"
	"bufio"
	"io"
)

var bufioReaderCache sync.Pool

func newBufioReader(r io.Reader) *bufio.Reader {
	if br, ok := bufioReaderCache.Get().(*bufio.Reader); ok {
		br.Reset(r)
		return br
	}
	return bufio.NewReader(r)
}

func putBufioReader(br *bufio.Reader) {
	br.Reset(nil)
	bufioReaderCache.Put(br)
}

type endlessListener struct {
	net.Listener
	stopped bool
	server  *endlessServer
}

func (el *endlessListener) Accept() (c net.Conn, err error) {
	tc, err := el.Listener.(*net.TCPListener).AcceptTCP()
	if err != nil {
		return
	}

	tc.SetKeepAlive(true)                  // see http.tcpKeepAliveListener
	tc.SetKeepAlivePeriod(3 * time.Minute) // see http.tcpKeepAliveListener

	var rd *bufio.Reader
	var head *Header

	if el.server.ParseProxyProtocol {
		rd = newBufioReader(tc)
		var headErr error
		head, headErr = ReadHeader(rd)
		if headErr != nil {
			// Eat the error, let this request continue
			log.Printf("Error reading proxy protocol header: %v\n", headErr)
		}
		c = &endlessConn{
			Conn:   tc,
			server: el.server,
			ProxyHeader: head,
			rd: rd,
		}
	} else {
		c = &endlessConn{
			Conn:   tc,
			server: el.server,
		}
	}


	log.Println("Adding endlessConn")
	el.server.wg.Add(1)
	return
}

func newEndlessListener(l net.Listener, srv *endlessServer) (el *endlessListener) {
	el = &endlessListener{
		Listener: l,
		server:   srv,
	}

	return
}

func (el *endlessListener) Close() error {
	if el.stopped {
		return syscall.EINVAL
	}

	el.stopped = true
	return el.Listener.Close()
}

func (el *endlessListener) File() *os.File {
	// returns a dup(2) - FD_CLOEXEC flag *not* set
	tl := el.Listener.(*net.TCPListener)
	fl, _ := tl.File()
	return fl
}
