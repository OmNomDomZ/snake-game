package model

import (
	pb "SnakeGame/model/proto"
	"net"
)

type Player struct {
	state         *pb.GameState
	config        *pb.GameConfig
	multicastConn *net.UDPConn
	unicastConn   *net.UDPConn
}

func NewPlayer(multicastConn *net.UDPConn, unicastConn *net.UDPConn) *Player {
	return &Player{
		multicastConn: multicastConn,
		unicastConn:   unicastConn,
	}
}
