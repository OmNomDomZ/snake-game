package ui

import (
	"SnakeGame/connection"
	"SnakeGame/model/master"
	pb "SnakeGame/model/proto"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"google.golang.org/protobuf/proto"
	"image/color"
	"math/rand"
	"net"
	"strconv"
	"time"
)

const CellSize = 20

var gameTicker *time.Ticker
var isRunning bool

// ShowMainMenu выводит главное меню
func ShowMainMenu(w fyne.Window, multConn *net.UDPConn) {
	title := widget.NewLabel("Добро пожаловать в Snake Game!")
	title.Alignment = fyne.TextAlignCenter

	newGameButton := widget.NewButton("Новая игра", func() {
		ShowGameConfig(w, multConn)
	})

	exitButton := widget.NewButton("Выход", func() {
		w.Close()
	})

	content := container.NewVBox(
		title,
		newGameButton,
		exitButton,
	)

	w.SetContent(container.NewCenter(content))
}

// ShowGameConfig настройки игры
func ShowGameConfig(w fyne.Window, multConn *net.UDPConn) {
	widthEntry := widget.NewEntry()
	widthEntry.SetText("25")
	heightEntry := widget.NewEntry()
	heightEntry.SetText("25")
	foodEntry := widget.NewEntry()
	foodEntry.SetText("3")
	delayEntry := widget.NewEntry()
	delayEntry.SetText("900")

	startButton := widget.NewButton("Начать игру", func() {
		width, _ := strconv.Atoi(widthEntry.Text)
		height, _ := strconv.Atoi(heightEntry.Text)
		food, _ := strconv.Atoi(foodEntry.Text)
		delay, _ := strconv.Atoi(delayEntry.Text)

		config := &pb.GameConfig{
			Width:        proto.Int32(int32(width)),
			Height:       proto.Int32(int32(height)),
			FoodStatic:   proto.Int32(int32(food)),
			StateDelayMs: proto.Int32(int32(delay)),
		}

		ShowGameScreen(w, config, multConn)
	})

	backButton := widget.NewButton("Назад", func() {
		ShowMainMenu(w, multConn)
	})

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Ширина поля", Widget: widthEntry},
			{Text: "Высота поля", Widget: heightEntry},
			{Text: "Количество еды", Widget: foodEntry},
			{Text: "Задержка (мс)", Widget: delayEntry},
		},
	}

	content := container.NewVBox(
		widget.NewLabelWithStyle("Настройки игры", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		form,
		startButton,
		backButton,
	)

	w.SetContent(container.NewCenter(content))
}

// ShowGameScreen показывает экран игры
func ShowGameScreen(w fyne.Window, config *pb.GameConfig, multConn *net.UDPConn) {
	masterNode := master.NewMaster(multConn, config)
	go masterNode.Start()

	gameContent := CreateGameContent(config)

	scoreLabel := widget.NewLabel("Счет: 0")
	infoPanel := createInfoPanel(config, func() {
		StopGameLoop()
		ShowMainMenu(w, multConn)
	}, scoreLabel)

	splitContent := container.NewHSplit(
		gameContent,
		infoPanel,
	)
	splitContent.SetOffset(0.7)

	w.SetContent(splitContent)

	StartGameLoop(w, masterNode, gameContent, func(score int32) {
		scoreLabel.SetText(fmt.Sprintf("Счет: %d", score))
	})
}

// CreateGameContent создает холст
func CreateGameContent(config *pb.GameConfig) *fyne.Container {
	gameContent := fyne.NewContainerWithoutLayout()

	windowWidth := float32(config.GetWidth()) * CellSize
	windowHeight := float32(config.GetHeight()) * CellSize
	gameContent.Resize(fyne.NewSize(windowWidth, windowHeight))

	return gameContent
}

// StartGameLoop главный цикл игры
func StartGameLoop(w fyne.Window, masterNode *master.Master, gameContent *fyne.Container, updateScore func(int32)) {
	rand.NewSource(time.Now().UnixNano())

	gameTicker = time.NewTicker(time.Millisecond * 120)

	isRunning = true

	// обработка клавиш
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		handleKeyInput(e, masterNode)
	})

	go func() {
		for isRunning {
			select {
			case <-gameTicker.C:
				masterNode.Node.Mu.Lock()
				// обновления состояния игры
				masterNode.GenerateFood()
				masterNode.UpdateGameState()
				stateCopy := proto.Clone(masterNode.Node.State).(*pb.GameState)
				configCopy := proto.Clone(masterNode.Node.Config).(*pb.GameConfig)
				// обновление счета
				var playerScore int32
				for _, player := range masterNode.Node.State.GetPlayers().GetPlayers() {
					if player.GetId() == masterNode.Node.PlayerInfo.GetId() {
						playerScore = player.GetScore()
						break
					}
				}
				updateScore(playerScore)
				renderGameState(gameContent, stateCopy, configCopy)
				masterNode.Node.Mu.Unlock()
			}
		}
	}()
}

// StopGameLoop остановка игры
func StopGameLoop() {
	if gameTicker != nil {
		gameTicker.Stop()
	}
	isRunning = false
}

// handleKeyInput обработка клавиш
func handleKeyInput(e *fyne.KeyEvent, masterNode *master.Master) {
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

	masterNode.Node.Mu.Lock()
	defer masterNode.Node.Mu.Unlock()

	for _, snake := range masterNode.Node.State.Snakes {
		if snake.GetPlayerId() == masterNode.Node.PlayerInfo.GetId() {
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

// createInfoPanel информационная панель
func createInfoPanel(config *pb.GameConfig, onExit func(), scoreLabel *widget.Label) *fyne.Container {
	data := [][]string{
		{"Name", "Score"},
		{"Player1", "0"},
	}

	scoreTable := widget.NewTable(
		func() (int, int) {
			return len(data), len(data[0])
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			cell.(*widget.Label).SetText(data[id.Row][id.Col])
		},
	)

	scoreTable.SetColumnWidth(0, 100)
	scoreTable.SetColumnWidth(1, 50)

	scrollableTable := container.NewScroll(scoreTable)
	scrollableTable.SetMinSize(fyne.NewSize(150, 100))

	gameInfo := widget.NewLabel(fmt.Sprintf("Текущая игра:\n\nРазмер: %dx%d\nЕда: %d", config.GetWidth(), config.GetHeight(), config.GetFoodStatic()))
	newGameButton := widget.NewButton("Новая игра", onExit)

	exitButton := widget.NewButton("Выйти", onExit)

	content := container.NewVBox(
		container.New(layout.NewPaddedLayout(), scoreLabel),
		container.New(layout.NewPaddedLayout(), scrollableTable),
		container.New(layout.NewPaddedLayout(), gameInfo),
		container.New(layout.NewPaddedLayout(), newGameButton),
		container.New(layout.NewPaddedLayout(), exitButton),
	)
	return content
}

// updateInfoPanel обновление инф панели
func updateInfoPanel(scoreTable *widget.Table, state *pb.GameState) {
	data := [][]string{
		{"Name", "Score"},
	}
	for _, player := range state.GetPlayers().GetPlayers() {
		data = append(data, []string{player.GetName(), fmt.Sprintf("%d", player.GetScore())})
	}

	scoreTable.Length = func() (int, int) {
		return len(data), len(data[0])
	}
	scoreTable.UpdateCell = func(id widget.TableCellID, cell fyne.CanvasObject) {
		cell.(*widget.Label).SetText(data[id.Row][id.Col])
	}
	scoreTable.Refresh()
}

// RunApp запуск (в main)
func RunApp() {
	myApp := app.New()
	myWindow := myApp.NewWindow("SnakeGame")
	myWindow.Resize(fyne.NewSize(800, 600))
	myWindow.CenterOnScreen()

	multConn := connection.Connection()

	ShowMainMenu(myWindow, multConn)

	myWindow.ShowAndRun()
}
