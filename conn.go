package endless
import (
	"net"
	"bufio"
	"sync"
	"log"
)

type endlessConn struct {
	net.Conn
	server *endlessServer
	ProxyHeader *Header
	rd          *bufio.Reader
	mu          sync.Mutex
}

func (c *endlessConn) RemoteAddr() net.Addr {
	if c.ProxyHeader != nil {
		return c.ProxyHeader.SrcAddr
	}
	return c.Conn.RemoteAddr()
}

func (c *endlessConn) Read(b []byte) (int, error) {
	c.mu.Lock()
	rd := c.rd
	c.mu.Unlock()
	if rd != nil {
		bn := rd.Buffered()
		// No data left in buffer so switch to underlying connection
		if bn == 0 {
			c.mu.Lock()
			c.rd = nil
			c.mu.Unlock()
			return c.Conn.Read(b)
		}
		// If reading less than buffered data then just let it go through
		// since we'll need to continue using the bufio reader
		if bn < len(b) {
			return rd.Read(b)
		}
		// Drain the bufio and switch to the connection directly. This will
		// read less data than requested, but that's allowed by the io.Reader
		// interface contract.
		n, err := rd.Read(b[:bn])
		if err == nil {
			c.mu.Lock()
			c.rd = nil
			c.mu.Unlock()
		}
		return n, err
	} else {
		return c.Conn.Read(b)
	}
}

func (c *endlessConn) Close() error {
	log.Println("Closing endlessConn")
	c.mu.Lock()
	rd := c.rd
	c.rd = nil
	c.mu.Unlock()
	if rd != nil {
		putBufioReader(rd)
	}
	err := c.Conn.Close()
	if err == nil {
		c.server.wg.Done()
	}
	return err
}