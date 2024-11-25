package master

import (
	pb "SnakeGame/model/proto"
	"fmt"
	"google.golang.org/protobuf/proto"
	"log"
	"math/rand"
)

// генерация еды
func (m *Master) generateFood() {
	// TODO: написать функцию которая вычисляет только живых игроков
	requireFood := m.node.Config.GetFoodStatic() + int32(len(m.node.State.Snakes))
	currentFood := int32(len(m.node.State.GetFoods()))

	if currentFood < requireFood {
		needNum := requireFood - currentFood
		for i := int32(0); i < needNum; i++ {
			coord := m.findEmptyCell()
			if coord != nil {
				m.node.State.Foods = append(m.node.State.Foods, coord)
			} else {
				log.Println("No empty cells available for new food.")
				break
			}
		}
	}
}

func (m *Master) findEmptyCell() *pb.GameState_Coord {
	numCells := m.node.Config.GetWidth() * m.node.Config.GetHeight()
	for attempts := int32(0); attempts < numCells; attempts++ {
		x := rand.Int31n(m.node.Config.GetWidth())
		y := rand.Int31n(m.node.Config.GetHeight())
		if m.isCellEmpty(x, y) {
			return &pb.GameState_Coord{X: proto.Int32(x), Y: proto.Int32(y)}
		}
	}
	return nil
}

func (m *Master) isCellEmpty(x, y int32) bool {
	for _, snake := range m.node.State.Snakes {
		for _, point := range snake.Points {
			if point.GetX() == x && point.GetY() == y {
				return false
			}
		}
	}

	for _, food := range m.node.State.Foods {
		if food.GetX() == x && food.GetY() == y {
			return false
		}
	}

	return true
}

// обновление состояния игры
func (m *Master) updateGameState() {
	for _, snake := range m.node.State.Snakes {
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
		newHead.X = proto.Int32(m.node.Config.GetWidth() - 1)
	} else if newHead.GetX() >= m.node.Config.GetWidth() {
		newHead.X = proto.Int32(0)
	}
	if newHead.GetY() < 0 {
		newHead.Y = proto.Int32(m.node.Config.GetHeight() - 1)
	} else if newHead.GetY() >= m.node.Config.GetHeight() {
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
	for i, food := range m.node.State.Foods {
		if head.GetX() == food.GetX() && head.GetY() == food.GetY() {
			m.node.State.Foods = append(m.node.State.Foods[:i], m.node.State.Foods[i+1:]...)
			return true
		}
	}
	return false
}

// проверяем столкновения с другими змеями
func (m *Master) checkCollisions() {
	heads := make(map[string]int32)

	for _, snake := range m.node.State.Snakes {
		head := snake.Points[0]
		point := fmt.Sprintf("%d,%d", head.GetX(), head.GetY())
		heads[point] = snake.GetPlayerId()
	}

	// проверяем, есть ли клетки с более чем одной головой
	for key := range heads {
		count := 0
		var crashedPlayers []int32
		for k, pid := range heads {
			if k == key {
				count++
				crashedPlayers = append(crashedPlayers, pid)
			}
		}
		// несколько голов на одной клетке -- все погибают
		if count > 1 {
			for _, pid := range crashedPlayers {
				m.killSnake(pid, pid)
			}
		}
	}

	// проверяем столкновения головы змейки с телом других змей
	for _, snake := range m.node.State.Snakes {
		head := snake.Points[0]
		for _, otherSnake := range m.node.State.Snakes {
			if otherSnake.GetPlayerId() == snake.GetPlayerId() {
				continue
			}
			for _, point := range otherSnake.Points {
				if point.GetX() == head.GetX() && point.GetY() == head.GetY() {
					m.killSnake(snake.GetPlayerId(), otherSnake.GetPlayerId())
					break
				}
			}
		}
	}
}

// убираем умершую змею
func (m *Master) killSnake(crashedPlayerId, killer int32) {
	for _, snake := range m.node.State.Snakes {
		if snake.GetPlayerId() == crashedPlayerId {
			for _, point := range snake.Points {
				if rand.Float32() < 0.5 {
					m.node.State.Foods = append(m.node.State.Foods, point)
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
