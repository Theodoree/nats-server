package server

import (
	"context"
	"github.com/xtaci/kcp-go"
	"net"
)

type BasicKcpService struct {
}

func (BasicKcpService) Dail(network string, raddr string) (net.Conn, error) {
	switch network {
	case "tcp", "kcp":
		return kcp.Dial(raddr)
	default:
		return net.Dial(network, raddr)
	}
}
func (BasicKcpService) Listen(network string, addr string) (net.Listener, error) {
	switch network {
	case "tcp", "kcp":
		return kcp.Listen(addr)
	default:
		return natsListenConfig.Listen(context.Background(), network, addr)
	}
}
