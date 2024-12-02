package ui

import (
	"SnakeGame/model/common"
	pb "SnakeGame/model/proto"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"image/color"
)

const CellSize = 20

// handleKeyInput обработка клавиш
func handleKeyInput(e *fyne.KeyEvent, node *common.Node) {
	var newDirection pb.Direction

	switch e.Name {
	case fyne.KeyW, fyne.KeyUp:
		newDirection = pb.Direction_UP
	case fyne.KeyS, fyne.KeyDown:
		newDirection = pb.Direction_DOWN
	case fyne.KeyA, fyne.KeyLeft:
		newDirection = pb.Direction_LEFT
	case fyne.KeyD, fyne.KeyRight:
		newDirection = pb.Direction_RIGHT
	default:
		return
	}

	node.Mu.Lock()
	defer node.Mu.Unlock()

	for _, snake := range node.State.Snakes {
		if snake.GetPlayerId() == node.PlayerInfo.GetId() {
			currentDirection := snake.GetHeadDirection()
			// проверка направления
			isOppositeDirection := func(cur, new pb.Direction) bool {
				switch cur {
				case pb.Direction_UP:
					return new == pb.Direction_DOWN
				case pb.Direction_DOWN:
					return new == pb.Direction_UP
				case pb.Direction_LEFT:
					return new == pb.Direction_RIGHT
				case pb.Direction_RIGHT:
					return new == pb.Direction_LEFT
				}
				return false
			}(currentDirection, newDirection)

			if !isOppositeDirection {
				snake.HeadDirection = newDirection.Enum()
			}
			break
		}
	}
}

// renderGameState выводит игру на экран
func renderGameState(content *fyne.Container, state *pb.GameState, config *pb.GameConfig) {
	content.Objects = nil

	// игровое поле
	for i := int32(0); i < config.GetWidth(); i++ {
		for j := int32(0); j < config.GetHeight(); j++ {
			cell := canvas.NewRectangle(color.RGBA{R: 50, G: 50, B: 50, A: 255})
			cell.StrokeColor = color.RGBA{R: 0, G: 0, B: 0, A: 255}
			cell.StrokeWidth = 1
			cell.Resize(fyne.NewSize(CellSize, CellSize))
			cell.Move(fyne.NewPos(float32(i)*CellSize, float32(j)*CellSize))
			content.Add(cell)
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

	// змеи
	for _, snake := range state.Snakes {
		for i, point := range snake.Points {
			var rect *canvas.Rectangle
			if i == 0 {
				// голова
				rect = canvas.NewRectangle(color.RGBA{0, 255, 0, 255})
			} else {
				// тело
				rect = canvas.NewRectangle(color.RGBA{0, 128, 0, 255})
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
