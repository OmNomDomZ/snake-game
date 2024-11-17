package model

import (
	pb "SnakeGame/model/proto"
	"google.golang.org/protobuf/proto"
)

func initPlayer() (*pb.GameState, *pb.GameConfig) {
	gameConfig := &pb.GameConfig{}

	player := &pb.GamePlayer{
		Role:  pb.NodeRole_NORMAL.Enum(),
		Type:  pb.PlayerType_HUMAN.Enum(),
		Score: proto.Int32(0),
	}

	players := &pb.GamePlayers{}

	snake := &pb.GameState_Snake{
		PlayerId: proto.Int32(player.GetId()),
		Points: []*pb.GameState_Coord{
			{X: proto.Int32(gameConfig.GetWidth() / 2), Y: proto.Int32(gameConfig.GetHeight() / 2)},
		},
		State:         pb.GameState_Snake_ALIVE.Enum(),
		HeadDirection: pb.Direction_RIGHT.Enum(),
	}

	foods := []*pb.GameState_Coord{}

	return gameConfig
}
