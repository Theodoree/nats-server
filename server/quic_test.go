package server

import (
	"fmt"
	"github.com/nats-io/nats.go"
	"testing"
)

func Test_quic_Dial(t *testing.T) {

	cli,err:=nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(cli.RTT())


}
