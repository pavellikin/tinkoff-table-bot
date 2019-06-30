package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
	"strings"
	"strconv"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"google.golang.org/api/sheets/v4"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

func getConfig() (*oauth2.Config) {
	credentialsString := os.Getenv("GOOGLE_CREDENTIALS")
	var credentialsBytes []byte
	if credentialsString != "" {
		credentialsBytes = []byte(credentialsString)
	} else {
		var err error
		credentialsBytes, err = ioutil.ReadFile("credentials.json")
		if err != nil {
			log.Fatalf("Unable to read client secret file: %v", err)
		}
	}
	config, err := google.ConfigFromJSON(credentialsBytes, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	return config
}

func getClient(config *oauth2.Config) *http.Client {
	tokenString := os.Getenv("SHEET_TOKEN")
	if tokenString != "" {
		token := &oauth2.Token{}
		err := json.Unmarshal([]byte(tokenString), token)
		if err != nil {
			log.Fatalf("Unable to unmarshall token %v", err)
		}
		return config.Client(context.Background(), token)
	}
	tokFile := "token.json"
	token, err := tokenFromFile(tokFile)
	if err != nil {
		token = getTokenFromWeb(config)
		saveToken(tokFile, token)
	}
	return config.Client(context.Background(), token)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)
	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}
	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func currentDate() (monthName string, day int) {
	_, month, day := time.Now().Date()
	switch month {
		case time.January: return "Январь", day
		case time.February: return "Февраль", day
		case time.March: return "Март", day
		case time.April: return "Апрель", day
		case time.May: return "Май", day
		case time.June: return "Июнь", day
		case time.July: return "Июль", day
		case time.August: return "Август", day
		case time.September: return "Сентябрь", day
		case time.October: return "Октябрь", day
		case time.November: return "Ноябрь", day
		case time.December: return "Декабрь", day
		default: return "", day
	}
}

func parseInput(input string) (description string, sum float64) {
	splitted := strings.Split(input, " ")
	var descriptionSlice [] string
	for _, word := range splitted {
		if value, err := strconv.ParseFloat(word, 64); err == nil {
			sum += value
			continue
		}
		descriptionSlice = append(descriptionSlice, word)
	}
	return strings.Join(descriptionSlice, ", "), sum
}

func updateTable(input string) (error) {
	description, sum := parseInput(input)
	config := getConfig()
	client := getClient(config)
	srv, err := sheets.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
		return err
	}
	spreadsheetID := os.Getenv("SHEET_ID")
	month, day := currentDate()
	workingRange := fmt.Sprintf("%s!H%d:I%d", month, day + 1, day + 1)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, workingRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
		return err
	}
	log.Println(resp.Values)
	var vr sheets.ValueRange
	var myValues []interface{}
	if len(resp.Values) == 0 {
		myValues = []interface{}{description, sum}
	} else {
		receicedKey := resp.Values[0][0].(string)
		receicedValue := resp.Values[0][1].(string)
		floatValue, err := strconv.ParseFloat(receicedValue, 64)
		if err != nil {
			return err
		}
		myValues = []interface{}{strings.ToLower(receicedKey+ ", " + description), floatValue + sum}
	}
	vr.Values = append(vr.Values, myValues)
	log.Println(vr.Values)
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, workingRange, &vr).ValueInputOption("RAW").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet. %v", err)
		return err
	}
	return nil
}

func main() {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_TOKEN"))
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := bot.GetUpdatesChan(u)
	for update := range updates {
		if update.Message == nil {
			continue
		}
		err := updateTable(update.Message.Text)
		var replyMessage tgbotapi.MessageConfig
		if err != nil {
			replyMessage = tgbotapi.NewMessage(update.Message.Chat.ID, "Some error accured")
		} else {
			replyMessage = tgbotapi.NewMessage(update.Message.Chat.ID, "Done")
		}
		replyMessage.ReplyToMessageID = update.Message.MessageID
		bot.Send(replyMessage)
	}
}