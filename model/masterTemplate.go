package model

import (
	pb "SnakeGame/model/proto"
	"google.golang.org/protobuf/proto"
)

func initMaster() (*pb.GameState, *pb.GameConfig, *pb.GameMessage_AnnouncementMsg) {
	gameConfig := &pb.GameConfig{
		Width:        proto.Int32(25),
		Height:       proto.Int32(25),
		FoodStatic:   proto.Int32(3),
		StateDelayMs: proto.Int32(200),
	}

	player := &pb.GamePlayer{
		Name:      proto.String("Player 1"),
		Id:        proto.Int32(1),
		IpAddress: nil,
		Port:      nil,
		Role:      pb.NodeRole_MASTER.Enum(),
		Type:      pb.PlayerType_HUMAN.Enum(),
		Score:     proto.Int32(0),
	}

	players := &pb.GamePlayers{
		Players: []*pb.GamePlayer{},
	}

	players.Players = append(players.Players, player)

	snake := &pb.GameState_Snake{
		PlayerId: proto.Int32(player.GetId()),
		Points: []*pb.GameState_Coord{
			{X: proto.Int32(gameConfig.GetWidth() / 2), Y: proto.Int32(gameConfig.GetHeight() / 2)},
		},
		State:         pb.GameState_Snake_ALIVE.Enum(),
		HeadDirection: pb.Direction_RIGHT.Enum(),
	}

	foods := []*pb.GameState_Coord{}

	state := &pb.GameState{
		StateOrder: proto.Int32(1),
		Snakes:     []*pb.GameState_Snake{snake},
		Foods:      foods,
		Players:    players,
	}

	announcementMsg := &pb.GameMessage_AnnouncementMsg{
		Games: []*pb.GameAnnouncement{
			{
				Players:  players,
				Config:   gameConfig,
				CanJoin:  proto.Bool(true),
				GameName: proto.String("FirstGame"),
			},
		},
	}

	return state, gameConfig, announcementMsg
}
