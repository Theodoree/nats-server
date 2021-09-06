package server

import (
	"context"
	"github.com/nats-io/nats.go"
	"github.com/xtaci/kcp-go"
	"io"
	"net"
	"sync"
	"time"
)

type BasicKcpService struct {
}

const (
	magicNumber byte = 0x10 // DLE
)

func SetBasicKcpService(options *nats.Options) error {
	options.CustomDialer = BasicKcpService{}
	return nil
}

func (b BasicKcpService) Dial(network, raddr string) (net.Conn, error) {
	switch network {
	case "tcp", "kcp":
		conn, err := kcp.DialWithOptions(raddr, nil, 10, 3)
		if err != nil {
			return nil, err
		}
		_, _ = conn.Write([]byte{magicNumber}) // make sure service accept
		return _newConn(conn), nil
	default:
		return net.Dial(network, raddr)
	}
}
func (b BasicKcpService) Listen(network string, addr string) (net.Listener, error) {
	switch network {
	case "tcp", "kcp":
		l, err := kcp.ListenWithOptions(addr, nil, 10, 3)
		if err != nil {
			return nil, err
		}
		return _listener{l: l}, nil
	default:
		return natsListenConfig.Listen(context.Background(), network, addr)
	}
}

type _listener struct {
	l net.Listener
}

type _error struct {
	msg string
}

func (e _error) Temporary() bool {
	return true
}

func (e _error) Error() string {
	return e.msg
}



type _timeoutError struct{
	msg string
}


func (e _timeoutError) Timeout() bool {
	return true
}

func (e _timeoutError) Error() string {
	return e.msg
}


func (l _listener) Accept() (net.Conn, error) {
	conn, err := l.l.Accept()
	if err != nil {
		return nil, err
	}

	var buf [1]byte
	_ = conn.SetReadDeadline(time.Now().Add(time.Millisecond * 200))
	_, err = conn.Read(buf[:])
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetReadDeadline(time.Time{})

	if buf[0] != magicNumber {
		_ = conn.Close()
		return nil, _error{"未知链接"}
	}

	return _newConn(conn), nil
}

func (l _listener) Close() error {
	return l.l.Close()
}

func (l _listener) Addr() net.Addr {
	return l.l.Addr()
}

func getNetAddrPort(addr net.Addr) int {
	switch val := addr.(type) {
	case *net.TCPAddr:
		return val.Port
	case *net.UDPAddr:
		return val.Port
	case Addr:
		return val.Port
	default:
		return NewAddr(addr).Port
	}
}

type Addr struct {
	addr net.Addr
	IP   net.IP
	Port int
	Zone string // IPv6 scoped addressing zone
}

func NewAddr(addr net.Addr) Addr {
	var localAddr Addr

	localAddr.addr = addr
	switch val := addr.(type) {
	case *net.UDPAddr:
		localAddr.IP = val.IP
		localAddr.Port = val.Port
		localAddr.Zone = val.Zone
	case *net.TCPAddr:
		localAddr.IP = val.IP
		localAddr.Port = val.Port
		localAddr.Zone = val.Zone
	case *net.IPAddr:
		localAddr.IP = val.IP
		localAddr.Zone = val.Zone

	}

	return localAddr
}

func (a Addr) Network() string {
	if a.addr != nil {
		return a.addr.Network()
	}
	return ""
}
func (a Addr) String() string {
	if a.addr != nil {
		return a.addr.String()
	}
	return ""
}

const closeAction = 'c'
const heartbeat = 'h'

type _serviceConn struct {
	net.Conn
	sync.Once

	// 上层调用确保写入安全
	rd time.Time
	wd time.Time
}

// 我只确保,调用前不会超时
func (s *_serviceConn) Read(buf []byte) (int, error) {
	if !s.rd.IsZero(){
		if time.Now().After(s.rd){
			return 0,_timeoutError{
				"读取超时",
			}
		}
	}

	for {
		count, err := s.Conn.Read(buf[:])
		if err != nil  {
			return 0, io.EOF
		}
		if count >= 2 {
			switch {
			case buf[0] == magicNumber:
				switch buf[1] {
				case closeAction:
					return 0, io.EOF
				}
			default:
				return count, nil
			}
		}

		return count,nil
	}
}

// 我只确保,调用前不会超时
func (s *_serviceConn) Write(buf []byte) (int,error){
	if !s.wd.IsZero(){
		if time.Now().After(s.wd){
			return 0,_timeoutError{
				"写入超时",
			}
		}
	}
	return s.Conn.Write(buf)
}
func (s *_serviceConn) SetDeadline(t time.Time) error {
	err:=s.Conn.SetDeadline(t)
	if err == nil {
		s.rd = t
		s.wd = t
	}
	return err
}

func (s *_serviceConn) SetReadDeadline(t time.Time) error{
	err:=s.Conn.SetReadDeadline(t)
	if err == nil {
		s.rd = t
	}
	return err


}

func (s *_serviceConn) SetWriteDeadline(t time.Time) error{
	err:=s.Conn.SetWriteDeadline(t)
	if err == nil {
		s.wd = t
	}
	return err
}



func (s *_serviceConn) Close() error {
	var  buf [128]byte
	buf[0] = magicNumber
	buf[1] = closeAction
	_, _ = s.Write(buf[:])
	time.Sleep(time.Millisecond*300) // 确保写完
	return s.Conn.Close()
}

func _newConn(c net.Conn) net.Conn {
	sc := &_serviceConn{Conn: c}
	return sc
}