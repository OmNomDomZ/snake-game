package ui

import (
	pb "SnakeGame/model/proto"
	"fmt"
	"image/color"
	"math/rand"
	"time"

	"google.golang.org/protobuf/proto"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

const CellSize = 20

var gameTicker *time.Ticker
var isRunning bool

// func CreateGameContent(state *pb.GameState, config *pb.GameConfig) *fyne.Container {
func CreateGameContent(field *field) *fyne.Container {
	gameContent := fyne.NewContainerWithoutLayout()

	windowWidth := float32(field.width) * CellSize
	windowHeight := float32(field.height) * CellSize
	gameContent.Resize(fyne.NewSize(windowWidth, windowHeight))

	return gameContent
}

// игровой цикл
func StartGameLoop(w fyne.Window, state *pb.GameState, config *pb.GameConfig, gameContent *fyne.Container, updateScore func(int32)) {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	gameTicker = time.NewTicker(time.Millisecond * time.Duration(config.GetStateDelayMs()))
	isRunning = true

	// обработка нажатий клавиш
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		handleKeyInput(e, state)
	})

	go func() {
		for isRunning {
			select {
			case <-gameTicker.C:
				updateGameState(state, config)
				renderGameState(gameContent, state, config)
				updateScore(state.GetPlayers().GetPlayers()[0].GetScore())
			}
		}
	}()
}

// остановка игрового цикла
func StopGameLoop() {
	if gameTicker != nil {
		gameTicker.Stop()
	}
	isRunning = false
}

// ввод с клавиатуры
func handleKeyInput(e *fyne.KeyEvent, state *pb.GameState) {
	switch e.Name {
	case fyne.KeyW, fyne.KeyUp:
		if state.Snakes[0].GetHeadDirection() != pb.Direction_DOWN {
			state.Snakes[0].HeadDirection = pb.Direction_UP.Enum()
		}
	case fyne.KeyS, fyne.KeyDown:
		if state.Snakes[0].GetHeadDirection() != pb.Direction_UP {
			state.Snakes[0].HeadDirection = pb.Direction_DOWN.Enum()
		}
	case fyne.KeyA, fyne.KeyLeft:
		if state.Snakes[0].GetHeadDirection() != pb.Direction_RIGHT {
			state.Snakes[0].HeadDirection = pb.Direction_LEFT.Enum()
		}
	case fyne.KeyD, fyne.KeyRight:
		if state.Snakes[0].GetHeadDirection() != pb.Direction_LEFT {
			state.Snakes[0].HeadDirection = pb.Direction_RIGHT.Enum()
		}
	}
}

// обновление состояния игры
func updateGameState(state *pb.GameState, config *pb.GameConfig) {
	for _, snake := range state.Snakes {
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
			newHead.X = proto.Int32(config.GetWidth() - 1)
		} else if newHead.GetX() >= config.GetWidth() {
			newHead.X = proto.Int32(0)
		}
		if newHead.GetY() < 0 {
			newHead.Y = proto.Int32(config.GetHeight() - 1)
		} else if newHead.GetY() >= config.GetHeight() {
			newHead.Y = proto.Int32(0)
		}

		// проверка столкновения с собой
		collided := false
		for _, point := range snake.Points {
			if newHead.GetX() == point.GetX() && newHead.GetY() == point.GetY() {
				collided = true
				break
			}
		}

		if collided {
			// рестарт
			snake.Points = []*pb.GameState_Coord{
				{X: proto.Int32(config.GetWidth() / 2), Y: proto.Int32(config.GetHeight() / 2)},
			}
			snake.HeadDirection = pb.Direction_RIGHT.Enum()
			for _, player := range state.GetPlayers().GetPlayers() {
				if player.GetId() == snake.GetPlayerId() {
					player.Score = proto.Int32(0)
				}
			}
			break
		}

		// еда
		ateFood := false
		for i, food := range state.Foods {
			if newHead.GetX() == food.GetX() && newHead.GetY() == food.GetY() {
				state.Foods = append(state.Foods[:i], state.Foods[i+1:]...)
				for _, player := range state.GetPlayers().GetPlayers() {
					if player.GetId() == snake.GetPlayerId() {
						player.Score = proto.Int32(player.GetScore() + 1)
					}
				}
				ateFood = true
				state.Foods = append(state.Foods, generateNewFood(state, config))
				break
			}
		}

		snake.Points = append([]*pb.GameState_Coord{newHead}, snake.Points...)
		if !ateFood {
			snake.Points = snake.Points[:len(snake.Points)-1]
		}
	}
}

// генерация еды
func generateNewFood(state *pb.GameState, config *pb.GameConfig) *pb.GameState_Coord {
	var x, y int32
	occupied := make(map[string]bool)

	for _, snake := range state.Snakes {
		for _, point := range snake.Points {
			key := fmt.Sprintf("%d,%d", point.GetX(), point.GetY())
			occupied[key] = true
		}
	}

	for _, food := range state.Foods {
		key := fmt.Sprintf("%d,%d", food.GetX(), food.GetY())
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

// отрисовка
func renderGameState(content *fyne.Container, state *pb.GameState, config *pb.GameConfig) {
	content.Objects = nil

	// земля
	for i := int32(0); i < config.GetWidth(); i++ {
		for j := int32(0); j < config.GetHeight(); j++ {
			grass := canvas.NewRectangle(color.RGBA{98, 84, 122, 255}) // Темно-зеленый
			grass.Resize(fyne.NewSize(CellSize, CellSize))
			grass.Move(fyne.NewPos(float32(i)*CellSize, float32(j)*CellSize))
			content.Add(grass)
		}
	}

	// еда
	for _, food := range state.Foods {
		apple := canvas.NewCircle(color.RGBA{255, 0, 0, 255})
		apple.Resize(fyne.NewSize(CellSize, CellSize))
		x := float32(food.GetX()) * CellSize
		y := float32(food.GetY()) * CellSize
		apple.Move(fyne.NewPos(x, y))
		content.Add(apple)
	}

	// змейка
	for _, snake := range state.Snakes {
		for i, point := range snake.Points {
			var rect *canvas.Rectangle
			if i == 0 {
				rect = canvas.NewRectangle(color.RGBA{0, 255, 0, 255}) // Голова змейки
			} else {
				rect = canvas.NewRectangle(color.RGBA{0, 128, 0, 255}) // Тело змейки
			}
			rect.Resize(fyne.NewSize(CellSize, CellSize))
			x := float32(point.GetX()) * CellSize
			y := float32(point.GetY()) * CellSize
			rect.Move(fyne.NewPos(x, y))
			content.Add(rect)
		}
	}

	content.Refresh()
}
