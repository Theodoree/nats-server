package server

import (
	"fmt"
	"github.com/nats-io/nats.go"
	"testing"
)

func TestBasicKcpService_Dail(t *testing.T) {
	client,err:=nats.Connect(fmt.Sprintf("%s:%d", DEFAULT_HOST, DEFAULT_PORT), SetBasicKcpService)
	if err != nil {
		t.Fatal(err)
	}

	for i:=0;i<10;i++{
		rtt,err:=client.RTT()
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(rtt)
	}
}
