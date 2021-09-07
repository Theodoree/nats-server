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


func SetBasicKcpService(options *nats.Options) error {
	options.CustomDialer = BasicKcpService{}
	return nil
}

func (b BasicKcpService) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error){
	return b.Dial(network,address)
}
func (b BasicKcpService) Dial(network, raddr string) (net.Conn, error) {
	switch network {
	case "tcp", "kcp":
		conn, err := kcp.DialWithOptions(raddr, nil, 10, 3)
		if err != nil {
			return nil, err
		}
		_, _ = conn.Write([]byte{magicNumber}) // make sure service accept
		return _newKcpConn(conn), nil
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
		return _kcp_listener{Listener: l}, nil
	default:
		return natsListenConfig.Listen(context.Background(), network, addr)
	}
}

type _kcp_listener struct {
	net.Listener
}
func (l _kcp_listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
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

	return _newKcpConn(conn), nil
}


const closeAction = 'c'
const heartbeat = 'h'

type _KcpConn struct {
	net.Conn
	sync.Once

	// 上层调用确保写入安全
	rd time.Time
	wd time.Time
}

// 我只确保,调用前不会超时
func (s *_KcpConn) Read(buf []byte) (int, error) {
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
func (s *_KcpConn) Write(buf []byte) (int,error){
	if !s.wd.IsZero(){
		if time.Now().After(s.wd){
			return 0,_timeoutError{
				"写入超时",
			}
		}
	}
	return s.Conn.Write(buf)
}
func (s *_KcpConn) SetDeadline(t time.Time) error {
	err:=s.Conn.SetDeadline(t)
	if err == nil {
		s.rd = t
		s.wd = t
	}
	return err
}
func (s *_KcpConn) SetReadDeadline(t time.Time) error{
	err:=s.Conn.SetReadDeadline(t)
	if err == nil {
		s.rd = t
	}
	return err


}
func (s *_KcpConn) SetWriteDeadline(t time.Time) error{
	err:=s.Conn.SetWriteDeadline(t)
	if err == nil {
		s.wd = t
	}
	return err
}
func (s *_KcpConn) Close() error {
	var  buf [128]byte
	buf[0] = magicNumber
	buf[1] = closeAction
	_, _ = s.Write(buf[:])
	time.Sleep(time.Millisecond*300) // 确保写完
	return s.Conn.Close()
}

func _newKcpConn(c net.Conn) net.Conn {
	sc := &_KcpConn{Conn: c}
	return sc
}