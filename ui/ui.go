package ui

import (
	"SnakeGame/connection"
	"SnakeGame/model/master"
	pb "SnakeGame/model/proto"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"google.golang.org/protobuf/proto"
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
	widthEntry.SetText("15")
	heightEntry := widget.NewEntry()
	heightEntry.SetText("15")
	foodEntry := widget.NewEntry()
	foodEntry.SetText("5")
	delayEntry := widget.NewEntry()
	delayEntry.SetText("180")

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
	infoPanel, scoreTable, foodCountLabel := createInfoPanel(config, func() {
		StopGameLoop()
		ShowMainMenu(w, multConn)
	}, scoreLabel)

	splitContent := container.NewHSplit(
		gameContent,
		infoPanel,
	)
	splitContent.SetOffset(0.7)

	w.SetContent(splitContent)

	StartGameLoop(w, masterNode, gameContent, scoreTable, foodCountLabel, func(score int32) {
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
func StartGameLoop(w fyne.Window, masterNode *master.Master, gameContent *fyne.Container,
	scoreTable *widget.Table, foodCountLabel *widget.Label, updateScore func(int32)) {
	rand.NewSource(time.Now().UnixNano())

	gameTicker = time.NewTicker(time.Millisecond * 60)

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
				//masterNode.GenerateFood()
				//masterNode.UpdateGameState()
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
				updateInfoPanel(scoreTable, foodCountLabel, stateCopy)
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

// createInfoPanel информационная панель
func createInfoPanel(config *pb.GameConfig, onExit func(), scoreLabel *widget.Label) (*fyne.Container, *widget.Table, *widget.Label) {
	data := [][]string{
		{"Name", "Score"},
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

	gameInfo := widget.NewLabel(fmt.Sprintf("Текущая игра:\n\nРазмер: %dx%d\n", config.GetWidth(), config.GetHeight()))
	foodCountLabel := widget.NewLabel("Еда: 0")

	newGameButton := widget.NewButton("Новая игра", onExit)
	exitButton := widget.NewButton("Выйти", onExit)

	content := container.NewVBox(
		container.New(layout.NewPaddedLayout(), scoreLabel),
		container.New(layout.NewPaddedLayout(), scrollableTable),
		container.New(layout.NewPaddedLayout(), gameInfo),
		container.New(layout.NewPaddedLayout(), foodCountLabel),
		container.New(layout.NewPaddedLayout(), newGameButton),
		container.New(layout.NewPaddedLayout(), exitButton),
	)

	return content, scoreTable, foodCountLabel
}

// updateInfoPanel обновление инф панели
func updateInfoPanel(scoreTable *widget.Table, foodCountLabel *widget.Label, state *pb.GameState) {
	data := [][]string{
		{"Name", "Score"},
	}
	for _, player := range state.GetPlayers().GetPlayers() {
		data = append(data, []string{player.GetName(), fmt.Sprintf("%d", player.GetScore())})
	}

	// обновляем таблицу счета
	scoreTable.Length = func() (int, int) {
		return len(data), len(data[0])
	}
	scoreTable.UpdateCell = func(id widget.TableCellID, cell fyne.CanvasObject) {
		cell.(*widget.Label).SetText(data[id.Row][id.Col])
	}
	scoreTable.Refresh()

	// обновляем количество еды
	foodCountLabel.SetText(fmt.Sprintf("Еда: %d", len(state.Foods)))
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
