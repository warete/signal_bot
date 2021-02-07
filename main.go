package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	apiUrl           = "https://api.probit.com/api/exchange/v1/"
	tickerMethodName = "ticker"
)

const (
	tgBotApiUrl = "https://api.telegram.org/bot"
	tgBotMethod = "sendMessage"
)

var (
	tgBotToken         string  = ""
	tgBotChatId        int     = 0
	watchingTickerPart string  = ""
	changeMinVal       float64 = 0.01
)
var oldData = make(map[string]ProbitTickerItem)

type ProbitTickerItem struct {
	Last             string `json:"last"`
	MarketId         string `json:"market_id"`
	LastConverted    float64
	ChangeCalculated float64
}

type ProbitResponse struct {
	Data []ProbitTickerItem `json:"data"`
}

func getTickersData() (*ProbitResponse, error) {
	client := &http.Client{}
	request, err := http.NewRequest("GET", apiUrl+tickerMethodName, nil)

	if err != nil {
		return nil, err
	}

	request.Header.Set("x-requested-with", "XMLHttpRequest")
	result, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	defer result.Body.Close()

	respData, err := ioutil.ReadAll(result.Body)
	if err != nil {
		return nil, err
	}

	tickerData := &ProbitResponse{}
	err = json.Unmarshal(respData, tickerData)
	if err != nil {
		return nil, err
	}

	return tickerData, nil
}

func processItemData(item ProbitTickerItem, tickerChan chan ProbitTickerItem) {
	var err error = nil
	item.LastConverted, err = strconv.ParseFloat(item.Last, 64)
	if err != nil {
		panic(err)
	}
	oldDataItem, oldDataExists := oldData[item.MarketId]
	if !oldDataExists {
		tickerChan <- item
		return
	}
	item.ChangeCalculated = math.Abs(item.LastConverted/oldDataItem.LastConverted - 1)
	if item.ChangeCalculated > changeMinVal {
		fmt.Println(item)
		sendTgMessage("["+item.MarketId+"] change "+strconv.FormatFloat(item.ChangeCalculated, 'f', 5, 64), 0)
	}
	tickerChan <- item
}

func sendTgMessage(message string, chatId int) {
	if chatId == 0 {
		if tgBotChatId == 0 {
			fmt.Println("Empty tg chat id")
			return
		}
		chatId = tgBotChatId
	}

	if len(tgBotToken) == 0 {
		fmt.Println("Empty tg bot api token")
		return
	}

	client := &http.Client{}
	data := url.Values{}
	data.Set("chat_id", strconv.Itoa(chatId))
	data.Set("text", message)
	request, err := http.NewRequest("GET", tgBotApiUrl+tgBotToken+"/"+tgBotMethod+"?"+data.Encode(), nil)
	if err != nil {
		fmt.Println(err)
	}
	request.Header.Set("x-requested-with", "XMLHttpRequest")
	result, err := client.Do(request)
	if err != nil {
		fmt.Println(err)
	}

	defer result.Body.Close()
}

func processCmdArgs() {
	flag.StringVar(&tgBotToken, "tg-token", "", "Telegram bot api token")
	flag.IntVar(&tgBotChatId, "tg-chatid", 0, "Telegram chat id")
	flag.StringVar(&watchingTickerPart, "ticker", "", "Ticker last part")
	flag.Float64Var(&changeMinVal, "change-min", 0.01, "Change min val")

	flag.Parse()

	if tgBotChatId == 0 {
		panic("Empty tg chat id")
	}
	if len(tgBotToken) == 0 {
		panic("Empty tg bot api token")
	}
}

func main() {
	processCmdArgs()

	for i := 0; ; i++ {
		mainTickerDataChan := make(chan ProbitResponse)
		go func(mainTickerDataChan chan ProbitResponse) {
			fmt.Println("start iter " + strconv.Itoa(i))
			tickersData, err := getTickersData()
			if err != nil {
				fmt.Println(err)
			}
			mainTickerDataChan <- *tickersData

		}(mainTickerDataChan)

		tickersData := ProbitResponse{}
		tickersData = <-mainTickerDataChan
		for _, item := range tickersData.Data {
			if len(item.Last) == 0 {
				continue
				fmt.Println("Empty last value: " + item.Last)
			}

			if len(watchingTickerPart) > 0 && !strings.Contains(item.MarketId, "-"+watchingTickerPart) {
				continue
			}
			tickerUpdateOldChan := make(chan ProbitTickerItem)
			go processItemData(item, tickerUpdateOldChan)
			oldItemData := <-tickerUpdateOldChan
			if len(oldItemData.MarketId) > 0 {
				oldData[oldItemData.MarketId] = oldItemData
			}
		}
		time.Sleep(time.Duration(5) * time.Second)
	}
}
