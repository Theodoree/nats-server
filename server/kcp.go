package server

import (
	"context"
	"github.com/xtaci/kcp-go"
	"net"
)

type BasicKcpService struct {
}

func (BasicKcpService) Dial(network, raddr string) (net.Conn, error){
	switch network {
	case "tcp", "kcp":
		conn,err:= kcp.DialWithOptions(raddr,nil,10,3)
		if err != nil {
			return nil,err
		}
		_,_ = conn.Write([]byte(pingProto)) // make sure service accept
		return conn,nil
	default:
		return net.Dial(network, raddr)
	}
}
func (BasicKcpService) Listen(network string, addr string) (net.Listener, error) {
	switch network {
	case "tcp", "kcp":
		return kcp.ListenWithOptions(addr,nil,10,3)
	default:
		return natsListenConfig.Listen(context.Background(), network, addr)
	}
}
