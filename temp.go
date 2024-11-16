package main

import (
	"SnakeGame/model"
	pb "SnakeGame/model/proto"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"google.golang.org/protobuf/proto"
	"image/color"
	"sync"
)

type GameController struct {
	mu         sync.Mutex
	gameState  *pb.GameState
	gameConfig *pb.GameConfig
	gameCanvas *fyne.Container
}

func NewGameController() *GameController {
	state, config := model.InitializeGame()

	return &GameController{
		gameState:  state,
		gameConfig: config,
		gameCanvas: container.NewWithoutLayout(),
	}
}

func (gc *GameController) DrawGameBoard() *fyne.Container {
	board := container.NewWithoutLayout()

	// Draw grid (optional, for visual clarity)
	for x := int32(0); x < gc.gameConfig.GetWidth(); x++ {
		for y := int32(0); y < gc.gameConfig.GetHeight(); y++ {
			rect := canvas.NewRectangle(color.NRGBA{R: 200, G: 200, B: 200, A: 255})
			rect.Resize(fyne.NewSize(20, 20))
			rect.Move(fyne.NewPos(float32(x)*20, float32(y)*20))
			board.Add(rect)
		}
	}

	// Draw snakes and food
	gc.DrawSnakes(board)
	gc.DrawFood(board)

	gc.gameCanvas = board
	return board
}

func (gc *GameController) DrawSnakes(board *fyne.Container) {
	for _, snake := range gc.gameState.Snakes {
		for _, point := range snake.Points {
			rect := canvas.NewRectangle(color.NRGBA{R: 0, G: 255, B: 0, A: 255}) // Green for snake body
			rect.Resize(fyne.NewSize(20, 20))
			rect.Move(fyne.NewPos(float32(point.GetX())*20, float32(point.GetY())*20))
			board.Add(rect)
		}
	}
}

func (gc *GameController) DrawFood(board *fyne.Container) {
	for _, food := range gc.gameState.Foods {
		circle := canvas.NewCircle(color.NRGBA{R: 255, G: 0, B: 0, A: 255}) // Red for food
		circle.Resize(fyne.NewSize(20, 20))
		circle.Move(fyne.NewPos(float32(food.GetX())*20, float32(food.GetY())*20))
		board.Add(circle)
	}
}

func (gc *GameController) Update() {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	// Example: Move the snake head for demonstration purposes
	if len(gc.gameState.Snakes) > 0 {
		snake := gc.gameState.Snakes[0]
		head := snake.Points[0]

		// Calculate new head position (moving right as an example)
		newHead := &pb.GameState_Coord{
			X: proto.Int32(head.GetX() + 1),
			Y: proto.Int32(head.GetY()),
		}

		// Update the snake
		snake.Points = append([]*pb.GameState_Coord{newHead}, snake.Points[:len(snake.Points)-1]...)
	}

	// Refresh the canvas
	gc.gameCanvas.Objects = nil
	gc.DrawGameBoard()
}

//func main() {
//	// Create a Fyne app and window
//	a := app.New()
//	w := a.NewWindow("Snake Game")
//
//	// Initialize the game controller
//	controller := NewGameController()
//	board := controller.DrawGameBoard()
//
//	// Start button to initiate the game loop
//	startButton := widget.NewButton("Start", func() {
//		go func() {
//			ticker := time.NewTicker(time.Millisecond * time.Duration(controller.gameConfig.GetStateDelayMs()))
//			defer ticker.Stop()
//
//			for range ticker.C {
//				controller.Update()
//				w.Content().Refresh()
//			}
//		}()
//	})
//
//	// Layout the content
//	content := container.NewVBox(
//		board,
//		startButton,
//	)
//
//	// Set up the window
//	w.SetContent(content)
//	w.Resize(fyne.NewSize(800, 600))
//	w.ShowAndRun()
//}
