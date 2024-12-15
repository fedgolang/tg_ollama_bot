package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	ollama "github.com/ollama/ollama/api"
)

const (
	modelName = "llama3.1:8b"
)

var (
	greetMessage = "Привет, добро пожаловать в бота, который поможет тебе найти ответы на интересующие тебя вопросы!"
	aboutMessage = "Небольшой проект ТГ бота для личного развития в изучении GO, суть простая, под капотом у бота лежит llama3.1:8b, который отвечает на вопросы юзера"

	screaming = false
	bot       *tgbotapi.BotAPI

	ollamaUrl = "http://localhost:11434"
)

func init() {
	// Подгрузим env файл
	if err := godotenv.Load("./internal/config/.env"); err != nil {
		log.Print("No .env file found")
	}
}

func main() {
	var err error

	// Создаем коннект к боту
	bot, err = tgbotapi.NewBotAPI(os.Getenv("TG_TOKEN"))
	if err != nil {
		// Рухнем, если не удалось поднять бота
		log.Panic(err)
	}

	// llm лежит на локалхосте, запарсим УРЛ
	url, _ := url.Parse(ollamaUrl)

	// Новый клиент для работы с llm
	client := ollama.NewClient(url, http.DefaultClient)

	// Включим дебаг бота, чтобы последить что там может пойти не так
	bot.Debug = true

	// Повесим апдейт на бота и таймаут
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	defer cancel()

	updates := bot.GetUpdatesChan(u)

	go receiveUpdates(client, ctx, updates)

	log.Println("Жду обновлений")

	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

// Ждем сообщения от юзера
func receiveUpdates(client *ollama.Client, ctx context.Context, updates tgbotapi.UpdatesChannel) {
	// Бесконечный цикл, из которого будем выходить при отмене контекста
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			handleUpdate(client, update)
		}
	}

}

// Обрабатываем сообщение или команду
func handleUpdate(client *ollama.Client, update tgbotapi.Update) {
	switch {
	case update.Message != nil:
		handleMessage(client, update.Message)
		break
	}
}

// Обработаем сообщение
func handleMessage(client *ollama.Client, message *tgbotapi.Message) {
	user := message.From
	text := message.Text

	if user == nil {
		return
	}

	// Print to console
	log.Printf("%s написал %s", user.FirstName, text)

	var err error
	if strings.HasPrefix(text, "/") {
		err = handleCommand(message.Chat.ID, text)
	} else {
		ollamaResp, err := executeMessage(client, text)
		if err != nil {
			log.Printf("Ошибка выполнения запроса: %s", err.Error())
			return
		}
		msg := tgbotapi.NewMessage(message.Chat.ID, ollamaResp)
		_, err = bot.Send(msg)
	}

	if err != nil {
		log.Printf("Ошибка выполнения запроса: %s", err.Error())
	}

}

// Обработаем команду
func handleCommand(chatId int64, command string) error {
	var err error

	// Пройдётся по доступным командам
	switch command {
	case "/start":
		msg := tgbotapi.NewMessage(chatId, greetMessage)
		_, err = bot.Send(msg)
	case "/about":
		msg := tgbotapi.NewMessage(chatId, aboutMessage)
		_, err = bot.Send(msg)
	}

	return err
}

// Отправим LLM наше сообщение и получим ответ
func executeMessage(client *ollama.Client, msg string) (string, error) {
	var response string
	ctx := context.Background()

	req := &ollama.GenerateRequest{
		Model:  modelName,
		Prompt: msg,
	}

	err := client.Generate(ctx, req, func(resp ollama.GenerateResponse) error {
		response += resp.Response
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("Проблема выполнения запроса: %s", err)
	}

	return response, nil

}
