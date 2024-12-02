package ui

import (
	"SnakeGame/connection"
	"SnakeGame/model/common"
	"SnakeGame/model/master"
	"SnakeGame/model/player"
	pb "SnakeGame/model/proto"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"google.golang.org/protobuf/proto"
	"log"
	"math/rand"
	"net"
	"strconv"
	"time"
)

var gameTicker *time.Ticker
var isRunning bool

// ShowMainMenu выводит главное меню
func ShowMainMenu(w fyne.Window, multConn *net.UDPConn) {
	title := widget.NewLabel("Добро пожаловать в Snake Game!")
	title.Alignment = fyne.TextAlignCenter

	newGameButton := widget.NewButton("Новая игра", func() {
		ShowGameConfig(w, multConn)
	})

	joinGameButton := widget.NewButton("Присоединиться к игре", func() {
		ShowJoinGame(w, multConn)
	})

	exitButton := widget.NewButton("Выход", func() {
		w.Close()
	})

	content := container.NewVBox(
		title,
		newGameButton,
		joinGameButton,
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

		ShowMasterGameScreen(w, config, multConn)
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

// ShowMasterGameScreen показывает экран игры
func ShowMasterGameScreen(w fyne.Window, config *pb.GameConfig, multConn *net.UDPConn) {
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

	StartGameLoop(w, masterNode.Node, gameContent, scoreTable, foodCountLabel, func(score int32) {
		scoreLabel.SetText(fmt.Sprintf("Счет: %d", score))
	})
}

// ShowJoinGame отображает экран присоединения к игре
func ShowJoinGame(w fyne.Window, multConn *net.UDPConn) {
	log.Printf("присоединение...")
	playerNode := player.NewPlayer(multConn)
	go playerNode.ReceiveMulticastMessages()

	discoveryLabel := widget.NewLabel("Поиск доступных игр...")
	discoveryLabel.Alignment = fyne.TextAlignCenter

	gameList := widget.NewSelect([]string{}, func(value string) {
		// Здесь можно обработать выбор игры (при необходимости)
		log.Printf("Selected game: %s", value)
	})
	gameList.PlaceHolder = "Выберите игру"
	gameList.Resize(fyne.NewSize(300, 50))

	//gameList := widget.NewList(
	//	func() int { return 0 },
	//	func() fyne.CanvasObject {
	//		return widget.NewLabel("Loading...")
	//	},
	//	func(i widget.ListItemID, o fyne.CanvasObject) {
	//		o.(*widget.Label).SetText("Loading...")
	//	},
	//)

	playerNameEntry := widget.NewEntry()
	playerNameEntry.SetPlaceHolder("Введите ваше имя")

	joinButton := widget.NewButton("Присоединиться", func() {
		playerName := playerNameEntry.Text
		if playerName == "" {
			dialog := widget.NewLabel("Имя игрока не может быть пустым.")
			w.SetContent(container.NewCenter(dialog))
			return
		}
		// получаем выбранную игру из списка
		selectedGame := getSelectedGame(playerNode, gameList)
		if selectedGame != nil {
			ShowPlayerGameScreen(w, playerNode, playerName, selectedGame, multConn)
		}
	})

	backButton := widget.NewButton("Назад", func() {
		ShowMainMenu(w, multConn)
	})

	content := container.NewVBox(
		discoveryLabel,
		gameList,
		widget.NewForm(
			&widget.FormItem{Text: "Имя игрока", Widget: playerNameEntry},
		),
		joinButton,
		backButton,
	)

	w.SetContent(container.NewCenter(content))

	// Реализуем обнаружение игр и обновление списка
	go func() {
		games := playerNode.DiscoveredGames
		gameList.Options = getGameNames(games)
		gameList.Refresh()
		discoveryLabel.SetText("Выберите игру из списка")
	}()
}

func getGameNames(games []player.DiscoveredGame) []string {
	names := make([]string, len(games))
	for i, game := range games {
		names[i] = game.GameName
	}
	return names
}

func getSelectedGame(playerNode *player.Player, gameList *widget.Select) *player.DiscoveredGame {
	for _, game := range playerNode.DiscoveredGames {
		if gameList.Selected == game.GameName {
			return &game
		}
	}
	log.Printf("Could't find selected game")
	return nil
}

// ShowPlayerGameScreen инициализирует игрока и запускает UI игры
func ShowPlayerGameScreen(w fyne.Window, playerNode *player.Player, playerName string,
	selectedGame *player.DiscoveredGame, multConn *net.UDPConn) {

	playerNode.Node.PlayerInfo.Name = proto.String(playerName)
	playerNode.Node.Config = selectedGame.Config
	playerNode.MasterAddr = selectedGame.MasterAddr
	playerNode.AnnouncementMsg = selectedGame.AnnouncementMsg
	playerNode.Start()

	//gameContent := CreateGameContent(playerNode.Node.Config)
	//
	//scoreLabel := widget.NewLabel("Счет: 0")
	//infoPanel, scoreTable, foodCountLabel := createInfoPanel(playerNode.Node.Config, func() {
	//	StopGameLoop()
	//	ShowMainMenu(w, multConn)
	//}, scoreLabel)
	//
	//splitContent := container.NewHSplit(
	//	gameContent,
	//	infoPanel,
	//)
	//splitContent.SetOffset(0.7)
	//
	//w.SetContent(splitContent)
	//
	//StartGameLoop(w, playerNode.Node, gameContent, scoreTable, foodCountLabel, func(score int32) {
	//	scoreLabel.SetText(fmt.Sprintf("Счет: %d", score))
	//})
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
func StartGameLoop(w fyne.Window, node *common.Node, gameContent *fyne.Container,
	scoreTable *widget.Table, foodCountLabel *widget.Label, updateScore func(int32)) {
	rand.NewSource(time.Now().UnixNano())

	gameTicker = time.NewTicker(time.Millisecond * 60)

	isRunning = true

	// обработка клавиш
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		handleKeyInput(e, node)
	})

	if node.State == nil {
		node.Mu.Lock()
		for node.State == nil {
			node.Cond.Wait()
		}
		node.Mu.Unlock()
	}

	go func() {
		for isRunning {
			select {
			case <-gameTicker.C:
				node.Mu.Lock()
				stateCopy := proto.Clone(node.State).(*pb.GameState)
				configCopy := proto.Clone(node.Config).(*pb.GameConfig)
				// Обновление счёта
				var playerScore int32
				for _, player := range node.State.GetPlayers().GetPlayers() {
					if player.GetId() == node.PlayerInfo.GetId() {
						playerScore = player.GetScore()
						break
					}
				}
				updateScore(playerScore)
				renderGameState(gameContent, stateCopy, configCopy)
				updateInfoPanel(scoreTable, foodCountLabel, stateCopy)
				node.Mu.Unlock()
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
	for _, gamePlayer := range state.GetPlayers().GetPlayers() {
		data = append(data, []string{gamePlayer.GetName(), fmt.Sprintf("%d", gamePlayer.GetScore())})
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
