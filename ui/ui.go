package ui

import (
	"SnakeGame/game"
	"SnakeGame/model"
	pb "SnakeGame/model/proto"
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"google.golang.org/protobuf/proto"
)

// главное меню
func ShowMainMenu(w fyne.Window) {
	title := widget.NewLabel("Добро пожаловать в Snake Game!")
	title.Alignment = fyne.TextAlignCenter

	newGameButton := widget.NewButton("Новая игра", func() {
		ShowGameConfig(w)
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

// настройка новой игры
func ShowGameConfig(w fyne.Window) {
	widthEntry := widget.NewEntry()
	widthEntry.SetText("25")
	heightEntry := widget.NewEntry()
	heightEntry.SetText("25")
	foodEntry := widget.NewEntry()
	foodEntry.SetText("3")
	delayEntry := widget.NewEntry()
	delayEntry.SetText("200")

	startButton := widget.NewButton("Начать игру", func() {
		width, _ := strconv.Atoi(widthEntry.Text)
		height, _ := strconv.Atoi(heightEntry.Text)
		food, _ := strconv.Atoi(foodEntry.Text)
		delay, _ := strconv.Atoi(delayEntry.Text)

		state, config := model.InitializeGame()
		config.Width = proto.Int32(int32(width))
		config.Height = proto.Int32(int32(height))
		config.FoodStatic = proto.Int32(int32(food))
		config.StateDelayMs = proto.Int32(int32(delay))

		ShowGameScreen(w, state, config)
	})

	backButton := widget.NewButton("Назад", func() {
		ShowMainMenu(w)
	})

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Ширина поля", Widget: widthEntry},
			{Text: "Высота поля", Widget: heightEntry},
			{Text: "Количество еды", Widget: foodEntry},
			{Text: "Задержка (мс)", Widget: delayEntry},
		},
		OnSubmit: func() {},
	}

	content := container.NewVBox(
		widget.NewLabelWithStyle("Настройки игры", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		form,
		startButton,
		backButton,
	)

	w.SetContent(container.NewCenter(content))
}

// экран игры
func ShowGameScreen(w fyne.Window, state *pb.GameState, config *pb.GameConfig) {
	gameContent := game.CreateGameContent(state, config)

	infoPanel := createInfoPanel(state, config, func() {
		game.StopGameLoop()
		ShowMainMenu(w)
	})

	content := container.NewWithoutLayout(gameContent, infoPanel)
	updateGameContentSize(w, config, content, infoPanel)

	game.StartGameLoop(w, state, config, gameContent, func(score int32) {
		updateScore(infoPanel, score)
		updateInfoPanel(infoPanel, state, config)
	})

	w.SetContent(content)
	w.Resize(content.Size())
}

func updateGameContentSize(w fyne.Window, config *pb.GameConfig, content *fyne.Container, infoPanel *fyne.Container) {
	width := float32(config.GetWidth()) * game.CellSize
	height := float32(config.GetHeight()) * game.CellSize

	gameSize := fyne.NewSize(width+100, height)
	content.Resize(gameSize)
	infoPanel.Resize(fyne.NewSize(100, height))
	infoPanel.Move(fyne.NewPos(width, 0))
	w.Resize(gameSize)
}

func createInfoPanel(state *pb.GameState, config *pb.GameConfig, onExit func()) *fyne.Container {
	scoreLabel := widget.NewLabel(fmt.Sprintf("Счет: %d", state.GetPlayers().GetPlayers()[0].GetScore()))
	gameInfo := widget.NewLabel(fmt.Sprintf("Игра: Размер: %dx%d, Еда: %d", config.GetWidth(), config.GetHeight(), len(state.Foods)))

	exitButton := widget.NewButton("Выйти", onExit)
	newGameButton := widget.NewButton("Новая игра", onExit)

	content := container.NewVBox(
		gameInfo,
		scoreLabel,
		exitButton,
		newGameButton,
	)

	return content
}

func updateScore(infoPanel *fyne.Container, score int32) {
	scoreLabel := infoPanel.Objects[1].(*widget.Label)
	scoreLabel.SetText(fmt.Sprintf("Счет: %d", score))
}

func updateInfoPanel(infoPanel *fyne.Container, state *pb.GameState, config *pb.GameConfig) {
	gameInfo := infoPanel.Objects[0].(*widget.Label)
	gameInfo.SetText(fmt.Sprintf("Игра: Размер: %dx%d, Еда: %d", config.GetWidth(), config.GetHeight(), len(state.Foods)))
}
