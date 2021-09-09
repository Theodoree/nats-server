package server

import (
	"context"
	"crypto/tls"
	"github.com/lucas-clemente/quic-go"
	"net"
	"strings"
	"sync"
	"time"
)

const magicNumber = 0x74

type BasicQuicService struct {
}

func (q BasicQuicService) Listen(network, address string) (net.Listener, error) {
	return q.listen(network, address, nil)
}
func (q BasicQuicService) ListenTls(network, address string, tlsConfig *tls.Config) (net.Listener, error) {
	return q.listen(network, address, tlsConfig)
}
func (q BasicQuicService) listen(network, address string, tlsConfig *tls.Config) (net.Listener, error) {
	switch network {
	case "tcp", "udp":
		if tlsConfig == nil {
			tlsConfig = generateTLSConfig()
		}
		lis, err := quic.ListenAddr(address, tlsConfig, &quic.Config{
			HandshakeIdleTimeout: time.Second * 2,
		})
		if err != nil {
			return nil, err
		}
		return _quic_listener{lis, context.TODO()}, nil
	default:
		if tlsConfig != nil {
			return tls.Listen(network, address, tlsConfig)
		}
		return net.Listen(network, address)
	}
}

func (q BasicQuicService) Dial(network, address string) (net.Conn, error) {
	return q.dial(network, address, nil, 0)
}
func (q BasicQuicService) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	return q.dial(network, address, nil, timeout)
}
func (q BasicQuicService) DialTls(network, address string, tlsConfig *tls.Config) (net.Conn, error) {
	return q.dial(network, address, tlsConfig, 0)
}
func (q BasicQuicService) dial(network, address string, tlsConfig *tls.Config, timeout time.Duration) (net.Conn, error) {
	switch network {
	case "tcp", "udp":
		if tlsConfig == nil {
			tlsConfig = _dial_tls_config
		}
		address = strings.Replace(address,"0.0.0.0","127.0.0.1",-1)
		ss, err := quic.DialAddr(address, tlsConfig, &quic.Config{
			HandshakeIdleTimeout: timeout,
		})
		if err != nil {
			return nil, err
		}
		stream, err := ss.OpenStreamSync(context.TODO())
		if err != nil {
			return nil, err
		}

		conn:= _quic_conn{stream, ss}
		_,err = conn.Write([]byte{magicNumber})
		if err != nil{
			return nil,err
		}
		return conn, nil

	default:
		d := net.Dialer{
			Timeout:   timeout,
			KeepAlive: -1,
		}
		if tlsConfig != nil {
			return tls.DialWithDialer(&d, network, address, tlsConfig)
		}
		return d.Dial(network, address)
	}
}

type _quic_listener struct {
	quic.Listener
	ctx context.Context
}
func (l _quic_listener) Accept() (net.Conn, error) {
	session, err := l.Listener.Accept(l.ctx)
	if err != nil {
		return nil, err
	}
	stream, err := session.AcceptStream(context.Background())
	if err != nil {
		return nil, _error{err.Error()}
	}

	conn := _quic_conn{stream, session}

	var buf [1]byte
	_, err = conn.Read(buf[:])
	if err != nil {
		_ = conn.Close()
		return nil, _error{err.Error()}
	}

	if buf[0] != magicNumber {
		_ = conn.Close()
		return nil, _error{"未知链接"}
	}
	return conn, nil
}

func (l _quic_listener) Close()  error {
	return l.Listener.Close()

}
type _quic_conn struct {
	quic.Stream
	quic.Session
}

var _generateTLSConfigOnce sync.Once
var _tls_config  *tls.Config
var _dial_tls_config = &tls.Config{NextProtos: []string{"quic"},InsecureSkipVerify: true}

func generateTLSConfig() *tls.Config {
	_generateTLSConfigOnce.Do(func() {
		keyPEM := `-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQDZeCdD8i6YwDDIZUAo2UJoilyVj+lLHtu1ups+d+7lbPJR7LIS
ieL95X37G8ke+VQNcPEyiF29QDayyNT1Eo3wIvdpK/QmXCWDSK/RQYYLESJbZusG
YZhsVgIju4FuS56dndKbIBDiztpPnzgywoLth+VnR/bTgxSZiyegOoG6+wIDAQAB
AoGATCJknMUMyy195qqL68EkHrVR9IqNgl8rTFQoRZZ3bJrXuxbCwPrFHV5a3K69
mrpvUsVXq/lR2A/DFpR4+dOlHOfE8hVtbzBQNHmcySweHFijzxmACJtYQKIJfz6u
PAKuPTXXMZd9EA2JaA355wo5aSM12dHK93Ir8ih5kopOxAECQQD2LQYvbX9Oqv93
0cSsmmNSeJMQDiBt3rBzx0TMnmc91gSMU10b3J/TmoSsCXRJENjsHYI8PWHLbcqr
Kj9KcjdLAkEA4iXbutXdWS2yeqCbTLduuJd8rRk3DWxM6C/Jxr9h6UA1aGNPWQRV
be7cLyTej+8vyBDDwWfvbbSvOjDZN8/NEQJBAMdp8Xi52kZ/fjIxWn/3ED3eLkLz
LpHRsl4XLUQTjM4qb8S8QtAvB8kBgjdZ8Ti+zPl3begeUPnZFjNRJbPIkcECQHdp
QB3mgWtuWrivh3E5xmgH7VhFYTFgRzeuzB96vMt6EPlevu4lAKr8nhzyneZoiNVe
LM85/03xQzk5w+jZe9ECQBDYoYbW6tAwo5bwA8dHm2VXsQ8djCdDjBC3HURbYJxK
R2W2a2fIjCuMljDsgkDnzsoOK0GZOWzas4AfouU/cVE=
-----END RSA PRIVATE KEY-----`
		certPEM := `-----BEGIN CERTIFICATE-----
MIIBezCB5aADAgECAgEBMA0GCSqGSIb3DQEBCwUAMAAwIhgPMDAwMTAxMDEwMDAw
MDBaGA8wMDAxMDEwMTAwMDAwMFowADCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkC
gYEA2XgnQ/IumMAwyGVAKNlCaIpclY/pSx7btbqbPnfu5WzyUeyyEoni/eV9+xvJ
HvlUDXDxMohdvUA2ssjU9RKN8CL3aSv0Jlwlg0iv0UGGCxEiW2brBmGYbFYCI7uB
bkuenZ3SmyAQ4s7aT584MsKC7YflZ0f204MUmYsnoDqBuvsCAwEAAaMCMAAwDQYJ
KoZIhvcNAQELBQADgYEANh8YfpJixLaobnkCUDSHajmF0wyeMInNhVWLtYMyizET
T3RAjw3S+HNXs0JK9gvKNby959m/hEtM26pNfZHT1sUEGf46hpvVdfixhEWkyK4P
73vWDjLk++eE5jkU3VuxIGPEZsSNEUfzaMPz2SE46K4nsIEimWT4ETvZ7reXyVY=
-----END CERTIFICATE-----`
		tlsCert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if err != nil {
			panic(err)
		}
		_tls_config = &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			NextProtos:   []string{"quic"},
		}

	})
	return _tls_config
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
type _timeoutError struct {
	msg string
}
func (e _timeoutError) Timeout() bool {
	return true
}
func (e _timeoutError) Error() string {
	return e.msg
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

func ResolveAddr(network, address string) (Addr, error){
	switch network {
	case "tcp":
		a,_ :=net.ResolveUDPAddr("udp",address)
		return NewAddr(a),nil
	case "udp":
		a,_ :=net.ResolveUDPAddr("udp",address)
		return NewAddr(a),nil
	case "unix":
		a,_ :=net.ResolveUnixAddr("unix",address)
		return NewAddr(a),nil
	case "ip":
		a,_ :=net.ResolveIPAddr("unix",address)
		return NewAddr(a),nil
	default:
		return Addr{},nil
	}
}