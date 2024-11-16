package main

import (
	"SnakeGame/model"
	pb "SnakeGame/model/proto"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"google.golang.org/protobuf/proto"
	"image/color"
	"math/rand"
	"strconv"
	"time"
)

const cellSize = 20

func main() {
	rand.Seed(time.Now().UnixNano())

	state, config := model.InitializeGame()

	a := app.New()
	w := a.NewWindow("Snake Game")

	windowWidth := float32(config.GetWidth()) * cellSize
	windowHeight := float32(config.GetHeight()) * cellSize
	windowSize := fyne.NewSize(windowWidth, windowHeight+30)
	w.Resize(windowSize)

	scoreLabel := widget.NewLabel("Score: 0")

	gameContent := container.NewWithoutLayout()
	content := container.NewVBox(scoreLabel, gameContent)
	w.SetContent(content)

	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		handleKeyInput(e, state)
	})

	startGameLoop(state, config, gameContent, scoreLabel)

	w.ShowAndRun()
}

func handleKeyInput(e *fyne.KeyEvent, state *pb.GameState) {
	switch e.Name {
	case fyne.KeyW, fyne.KeyUp:
		state.Snakes[0].HeadDirection = pb.Direction_UP.Enum()
	case fyne.KeyS, fyne.KeyDown:
		state.Snakes[0].HeadDirection = pb.Direction_DOWN.Enum()
	case fyne.KeyA, fyne.KeyLeft:
		state.Snakes[0].HeadDirection = pb.Direction_LEFT.Enum()
	case fyne.KeyD, fyne.KeyRight:
		state.Snakes[0].HeadDirection = pb.Direction_RIGHT.Enum()
	}
}

func startGameLoop(state *pb.GameState, config *pb.GameConfig, gameContent *fyne.Container, scoreLabel *widget.Label) {
	go func() {
		ticker := time.NewTicker(time.Millisecond * time.Duration(config.GetStateDelayMs()))
		defer ticker.Stop()

		for range ticker.C {
			updateGameState(state, config)

			renderGameState(gameContent, state, scoreLabel, config)
		}
	}()
}

func updateGameState(state *pb.GameState, config *pb.GameConfig) {
	for _, snake := range state.Snakes {
		head := snake.Points[0]
		newHead := &pb.GameState_Coord{
			X: proto.Int32(head.GetX()),
			Y: proto.Int32(head.GetY()),
		}

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

		// проверяем врезались ли в себя
		for _, point := range snake.Points {
			if newHead.GetX() == point.GetX() && newHead.GetY() == point.GetY() {
				snake.Points = []*pb.GameState_Coord{
					{X: proto.Int32(config.GetWidth() / 2), Y: proto.Int32(config.GetHeight() / 2)},
				}
				snake.HeadDirection = pb.Direction_RIGHT.Enum()
				break
			}
		}

		ateFood := false
		for i, food := range state.Foods {
			if newHead.GetX() == food.GetX() && newHead.GetY() == food.GetY() {
				state.Foods = append(state.Foods[:i], state.Foods[i+1:]...)
				for _, player := range state.Players.Players {
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

func renderGameState(content *fyne.Container, state *pb.GameState, scoreLabel *widget.Label, config *pb.GameConfig) {
	content.Objects = nil

	for i := int32(0); i < config.GetWidth(); i++ {
		for j := int32(0); j < config.GetHeight(); j++ {
			grass := canvas.NewRectangle(color.RGBA{98, 84, 122, 255})
			grass.Resize(fyne.NewSize(cellSize, cellSize))
			grass.Move(fyne.NewPos(float32(i)*cellSize, float32(j)*cellSize))
			content.Add(grass)
		}
	}

	for _, food := range state.Foods {
		apple := canvas.NewCircle(color.RGBA{255, 0, 0, 255})
		apple.Resize(fyne.NewSize(cellSize, cellSize))
		x := float32(food.GetX()) * cellSize
		y := float32(food.GetY()) * cellSize
		apple.Move(fyne.NewPos(x, y))
		content.Add(apple)
	}

	for _, snake := range state.Snakes {
		for i, point := range snake.Points {
			var rect *canvas.Rectangle
			if i == 0 {
				rect = canvas.NewRectangle(color.RGBA{0, 255, 0, 255})
			} else {
				rect = canvas.NewRectangle(color.RGBA{0, 128, 0, 255})
			}
			rect.Resize(fyne.NewSize(cellSize, cellSize))
			x := float32(point.GetX()) * cellSize
			y := float32(point.GetY()) * cellSize
			rect.Move(fyne.NewPos(x, y))
			content.Add(rect)
		}
	}

	playerScore := state.Players.Players[0].GetScore()
	scoreLabel.SetText("Score: " + strconv.Itoa(int(playerScore)))

	content.Refresh()
}
