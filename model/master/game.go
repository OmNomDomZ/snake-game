package master

import (
	pb "SnakeGame/model/proto"
	"google.golang.org/protobuf/proto"
	"log"
	"math/rand"
)

// генерация еды
func (m *Master) generateFood() {
	requireFood := m.config.GetFoodStatic() + int32(len(m.players.GetPlayers()))
	currentFood := int32(len(m.state.GetFoods()))

	if currentFood < requireFood {
		needNum := requireFood - currentFood
		for i := int32(0); i < needNum; i++ {
			coord := m.findEmptyCell()
			if coord != nil {
				m.state.Foods = append(m.state.Foods, coord)
			} else {
				log.Println("No empty cells available for new food.")
				break
			}
		}
	}
}

func (m *Master) findEmptyCell() *pb.GameState_Coord {
	numCells := m.config.GetWidth() * m.config.GetHeight()
	for attempts := int32(0); attempts < numCells; attempts++ {
		x := rand.Int31n(m.config.GetWidth())
		y := rand.Int31n(m.config.GetHeight())
		if m.isCellEmpty(x, y) {

			break
		}
	}
	return nil
}

func (m *Master) isCellEmpty(x, y int32) bool {
	for _, snake := range m.state.Snakes {
		for _, point := range snake.Points {
			if point.GetX() == x && point.GetY() == y {
				return false
			}
		}
	}

	for _, food := range m.state.Foods {
		if food.GetX() == x && food.GetY() == y {
			return false
		}
	}

	return true
}

// обновление состояния игры
func (m *Master) updateGameState() {
	for _, snake := range m.state.Snakes {
		m.moveSnake(snake)
	}

	m.checkCollisions()
}

func (m *Master) moveSnake(snake *pb.GameState_Snake) {
	head := snake.Points[0]
	newHead := &pb.GameState_Coord{
		X: proto.Int32(head.GetX()),
		Y: proto.Int32(head.GetY()),
	}

	// изменение координат
	switch snake.GetHeadDirection() {
	case pb.Direction_UP:
		newHead.Y = proto.Int32(newHead.GetY() - 1)
	case pb.Direction_DOWN:
		newHead.Y = proto.Int32(newHead.GetY() + 1)
	case pb.Direction_LEFT:
		newHead.X = proto.Int32(newHead.GetX() - 1)
	case pb.Direction_RIGHT:
		newHead.X = proto.Int32(newHead.GetX() + 1)
	}

	// поведение при столкновении со стеной
	if newHead.GetX() < 0 {
		newHead.X = proto.Int32(m.config.GetWidth() - 1)
	} else if newHead.GetX() >= m.config.GetWidth() {
		newHead.X = proto.Int32(0)
	}
	if newHead.GetY() < 0 {
		newHead.Y = proto.Int32(m.config.GetHeight() - 1)
	} else if newHead.GetY() >= m.config.GetHeight() {
		newHead.Y = proto.Int32(0)
	}

	// добавляем новую голову
	snake.Points = append([]*pb.GameState_Coord{newHead}, snake.Points...)
	if !m.isFoodEaten(newHead) {
		snake.Points = snake.Points[:len(snake.Points)-1]
	} else {
		// игрок заработал +1 балл
		snakeId := snake.GetPlayerId()
		for _, player := range m.players.GetPlayers() {
			if player.GetId() == snakeId {
				player.Score = proto.Int32(player.GetScore() + 1)
				break
			}
		}
	}
}

func (m *Master) isFoodEaten(head *pb.GameState_Coord) bool {
	for i, food := range m.state.Foods {
		if head.GetX() == food.GetX() && head.GetY() == food.GetY() {
			m.state.Foods = append(m.state.Foods[:i], m.state.Foods[i+1:]...)
			return true
		}
	}
	return false
}

// проверяем столкновения с другими змеями
func (m *Master) checkCollisions() {
	heads := make(map[*pb.GameState_Coord]int32)

	for _, snake := range m.state.Snakes {
		head := snake.Points[0]
		point := pb.GameState_Coord{X: head.X, Y: head.Y}
		heads[&point] = snake.GetPlayerId()
	}

	for _, snake := range m.state.Snakes {
		for _, point := range snake.Points {
			for head, crashedPlayerId := range heads {
				if point.GetX() == head.GetX() && point.GetY() == head.GetY() {
					m.killSnake(crashedPlayerId, snake.GetPlayerId())
				}
			}
		}
	}
}

// убираем умершую змею
func (m *Master) killSnake(crashedPlayerId, killer int32) {
	for _, snake := range m.state.Snakes {
		if snake.GetPlayerId() == crashedPlayerId {
			for _, point := range snake.Points {
				if rand.Float32() < 0.5 {
					m.state.Foods = append(m.state.Foods, point)
				}
			}
		}
	}

	if crashedPlayerId != killer {
		for _, player := range m.players.Players {
			if player.GetId() == killer {
				player.Score = proto.Int32(player.GetScore() + 1)
				break
			}
		}
	}
}
