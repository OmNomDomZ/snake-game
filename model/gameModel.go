package model

import (
	pb "SnakeGame/model/proto"
	"fmt"
	"math/rand"

	"google.golang.org/protobuf/proto"
)

func InitializeGame() (*pb.GameState, *pb.GameConfig) {
	gameConfig := &pb.GameConfig{
		Width:        proto.Int32(25),
		Height:       proto.Int32(25),
		FoodStatic:   proto.Int32(3),
		StateDelayMs: proto.Int32(200),
	}

	player := &pb.GamePlayer{
		Name:  proto.String("Player 1"),
		Id:    proto.Int32(1),
		Role:  pb.NodeRole_NORMAL.Enum(),
		Type:  pb.PlayerType_HUMAN.Enum(),
		Score: proto.Int32(0),
	}

	players := &pb.GamePlayers{
		Players: []*pb.GamePlayer{player},
	}

	snake := &pb.GameState_Snake{
		PlayerId: proto.Int32(1),
		Points: []*pb.GameState_Coord{
			{X: proto.Int32(gameConfig.GetWidth() / 2), Y: proto.Int32(gameConfig.GetHeight() / 2)},
		},
		State:         pb.GameState_Snake_ALIVE.Enum(),
		HeadDirection: pb.Direction_RIGHT.Enum(),
	}

	foods := []*pb.GameState_Coord{}
	for i := int32(0); i < gameConfig.GetFoodStatic(); i++ {
		food := generateRandomFood(snake, gameConfig)
		foods = append(foods, food)
	}

	state := &pb.GameState{
		StateOrder: proto.Int32(1),
		Snakes:     []*pb.GameState_Snake{snake},
		Foods:      foods,
		Players:    players,
	}

	return state, gameConfig
}

func generateRandomFood(snake *pb.GameState_Snake, config *pb.GameConfig) *pb.GameState_Coord {
	var x, y int32
	occupied := make(map[string]bool)
	for _, point := range snake.Points {
		key := fmt.Sprintf("%d,%d", point.GetX(), point.GetY())
		occupied[key] = true
	}

	for {
		x = rand.Int31n(config.GetWidth())
		y = rand.Int31n(config.GetHeight())
		key := fmt.Sprintf("%d,%d", x, y)
		if !occupied[key] {
			break
		}
	}

	return &pb.GameState_Coord{
		X: proto.Int32(x),
		Y: proto.Int32(y),
	}
}
