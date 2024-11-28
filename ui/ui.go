package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"image/color"
	"strconv"
)

type field struct {
	width  int
	height int
	food   int
	delay  int
}

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

		field := field{
			width:  width,
			height: height,
			food:   food,
			delay:  delay,
		}

		ShowGameScreen(w, &field)
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
	}

	content := container.NewVBox(
		widget.NewLabelWithStyle("Настройки игры", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		form,
		startButton,
		backButton,
	)

	w.SetContent(container.NewCenter(content))
}

func ShowGameScreen(w fyne.Window, field *field) {
	// игровое поле
	gameCanvas := canvas.NewRectangle(color.RGBA{R: 0, G: 255, B: 0, A: 255})
	gameCanvas.SetMinSize(fyne.NewSize(400, 400)) // Устанавливаем минимальный размер

	// панель с информацией
	infoPanel := createInfoPanel(field, func() {
		ShowMainMenu(w)
	})

	// Разделение на две части
	splitContent := container.NewHSplit(
		gameCanvas,
		infoPanel,
	)
	// Размеры разделения: 70% слева, 30% справа
	splitContent.SetOffset(0.7)

	w.SetContent(splitContent)
}

func createInfoPanel(field *field, onExit func()) *fyne.Container {
	data := [][]string{
		{"Name", "Score"},
		{"player1", "25"},
		{"player2", "30"},
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

	scoreTable.SetColumnWidth(0, 200)
	scoreTable.SetColumnWidth(1, 100)

	scrollableTable := container.NewScroll(scoreTable)
	scrollableTable.SetMinSize(fyne.NewSize(300, 150))

	gameInfo := widget.NewLabel(fmt.Sprintf("Текущая игра:\n\nВедущий: Валера\nНазвание: %s\nРазмер: %dx%d\nЕда: %d", "Game", field.width, field.height, field.food))
	newGameButton := widget.NewButton("Новая игра", onExit)

	exitButton := widget.NewButton("Выйти", onExit)

	content := container.NewVBox(
		container.New(layout.NewPaddedLayout(), scrollableTable),
		container.New(layout.NewPaddedLayout(), gameInfo),
		container.New(layout.NewPaddedLayout(), newGameButton),
		container.New(layout.NewPaddedLayout(), exitButton),
	)
	return content
}

func updateInfoPanel(window *fyne.Window, field *field) {

}
