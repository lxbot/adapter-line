package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/lxbot/lxlib"
	"github.com/mitchellh/mapstructure"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"
)

type (
	M = map[string]interface{}
	Raw struct {
		EventType linebot.EventType
		Event     string
	}
)

var (
	ch *chan M
	startTime time.Time
	channelSecret string
	channelAccessToken string
	bot *linebot.Client
)

func Boot(c *chan M) {
	ch = c

	channelSecret = os.Getenv("LXBOT_LINE_CHANNEL_SECRET")
	if channelSecret == "" {
		log.Fatalln("invalid url:", "'LXBOT_LINE_CHANNEL_SECRET' にチャネルシークレットを設定してください")
	}
	channelAccessToken = os.Getenv("LXBOT_LINE_CHANNEL_ACCESS_TOKEN")
	if channelAccessToken == "" {
		log.Fatalln("invalid token:", "'LXBOT_LINE_CHANNEL_ACCESS_TOKEN' にチャネルアクセストークン（長期）を設定してください")
	}

	var err error
	bot, err = linebot.New(channelSecret, channelAccessToken)
	if err != nil {
		log.Fatalln("line-bot-sdk初期化エラー:", err)
	}

	startTime = time.Now()

	go listen()

	gob.Register(Raw{})
}

func listen() {
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(middleware.CORS())

	e.GET("/", getIndex)
	e.POST("/", postMessaging, isValidLineSignature)

	log.Fatal(e.Start(":1323"))
}

func getIndex(c echo.Context) error {
	ms :=  new(runtime.MemStats)
	runtime.ReadMemStats(ms)

	return c.HTML(http.StatusOK, `
<!doctype html>
<html>
<head>
<title>lxbot - adapter-line</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/water.css@2/out/water.min.css">
</head>
<body>
<h1><a href="https://lxbot.io">lxbot</a> - <a href="https://github.com/lxbot/adapter-line">adapter-line</a></h1>
<ul>
<li>heap: ` + strconv.FormatUint(ms.HeapAlloc / 1024, 10) + ` KB</li>
<li>sys: ` + strconv.FormatUint(ms.Sys / 1024, 10) + ` KB</li>
<li>goroutine: ` + strconv.Itoa(runtime.NumGoroutine()) + `</li>
<li>uptime: ` + time.Since(startTime).String() + `</li>
</ul>
</body>
</html>
`)
}

func postMessaging(c echo.Context) error {
	events, err := bot.ParseRequest(c.Request())
	if err != nil {
		log.Println("line-bot-sdk ParseRequest error:", err)
		return c.NoContent(http.StatusNotAcceptable)
	}

	for _, event := range events {
		switch event.Type {
		case linebot.EventTypeMessage:
			c.Response().After(func() {
				c.Response().Flush()
				sendToLxbot(linebot.EventTypeMessage, event)
			})
			break
		case linebot.EventTypePostback:
			c.Response().After(func() {
				c.Response().Flush()
				sendToLxbot(linebot.EventTypePostback, event)
			})
			break
		case linebot.EventTypeFollow:
			c.Response().After(func() {
				c.Response().Flush()
				sendToLxbot(linebot.EventTypeFollow, event)
			})
			break
		case linebot.EventTypeUnfollow:
			c.Response().After(func() {
				c.Response().Flush()
				sendToLxbot(linebot.EventTypeUnfollow, event)
			})
			break
		case linebot.EventTypeJoin:
			c.Response().After(func() {
				c.Response().Flush()
				sendToLxbot(linebot.EventTypeJoin, event)
			})
			break
		case linebot.EventTypeLeave:
			c.Response().After(func() {
				c.Response().Flush()
				sendToLxbot(linebot.EventTypeLeave, event)
			})
			break
		case linebot.EventTypeBeacon:
			c.Response().After(func() {
				c.Response().Flush()
				sendToLxbot(linebot.EventTypeBeacon, event)
			})
			break
		default:
			log.Printf("invalid line messaging webhook: %v", *event)
		}
	}

	// HACK: res.Afterを呼ぶにはNoContentではダメ
	return c.String(200, "OK")
}

func sendToLxbot(eventType linebot.EventType, event *linebot.Event) {
	log.Println("line-bot-sdk EventType:", eventType)

	b, err := event.MarshalJSON()
	if err != nil {
		log.Println("raw event marshal error:", err)
	}

	uid := event.Source.UserID
	profile, err := bot.GetProfile(uid).Do()
	if err != nil {
		log.Println("guest profile fetch error:", err)
	}

	lxmsg, err := lxlib.NewLXMessage(lxlib.M{})
	if err != nil {
		log.Println(err)
		return
	}
	lxmsg.User = lxlib.User{
		ID:   uid,
		Name: profile.DisplayName,
	}
	lxmsg.Room = lxlib.Room{
		ID:          event.Source.RoomID,
		Name:        "LINE",
		Description: "LINE",
	}
	lxmsg.Raw = Raw{
		EventType: eventType,
		Event:     string(b),
	}

	switch message := event.Message.(type) {
	case *linebot.TextMessage:
		lxmsg.Message = lxlib.Message{
			ID:          message.ID,
			Text:        message.Text,
			Attachments: nil,
		}
	case *linebot.ImageMessage:
		lxmsg.Message = lxlib.Message{
			ID:          message.ID,
			Text:        "[IMAGE MESSAGE]",
			Attachments: []lxlib.Attachment{
				{
					Url: message.OriginalContentURL,
					Description: "",
				},
			},
		}
	case *linebot.AudioMessage:
		lxmsg.Message = lxlib.Message{
			ID:          message.ID,
			Text:        "[AUDIO MESSAGE]",
			Attachments: []lxlib.Attachment{
				{
					Url: message.OriginalContentURL,
					Description: "",
				},
			},
		}
	case *linebot.VideoMessage:
		lxmsg.Message = lxlib.Message{
			ID:          message.ID,
			Text:        "[VIDEO MESSAGE]",
			Attachments: []lxlib.Attachment{
				{
					Url: message.OriginalContentURL,
					Description: "",
				},
			},
		}
	}

	msg, err := lxmsg.ToMap()
	log.Println("msg generated:", msg)

	*ch <- msg
}

func isValidLineSignature(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		body, err := ioutil.ReadAll(c.Request().Body)
		if err != nil {
			log.Println("post body read error:", err)
			return c.NoContent(http.StatusNotAcceptable)
		}

		xLineSignature := c.Request().Header.Get("X-Line-Signature")
		decoded, err := base64.StdEncoding.DecodeString(xLineSignature)
		if err != nil {
			log.Println("X-Line-Signature decode error:", err)
			return c.NoContent(http.StatusForbidden)
		}

		hash := hmac.New(sha256.New, []byte(channelSecret))
		hash.Write(body)
		if ok := hmac.Equal(hash.Sum(nil), decoded); !ok {
			log.Println("invalid X-Line-Signature")
			return c.NoContent(http.StatusUnauthorized)
		}

		log.Println("X-Line-Signature OK")
		c.Request().Body = ioutil.NopCloser(bytes.NewBuffer(body))
		return next(c)
	}
}

func parseMsg(msg M) (*lxlib.LXMessage, *linebot.Event, error) {
	lxmsg, err := lxlib.NewLXMessage(msg)
	if err != nil {
		log.Println("lxmessage parse error:", err)
		return nil, nil, err
	}
	raw := new(Raw)
	if err := mapstructure.Decode(lxmsg.Raw, raw); err != nil {
		log.Println("raw data decode error:", err)
		return nil, nil, err
	}
	lxmsg.Raw = raw

	event := new(linebot.Event)
	if err := event.UnmarshalJSON([]byte(raw.Event)); err != nil {
		log.Println("event decode error:", err)
		return nil, nil, err
	}
	return lxmsg, event, nil
}

func Send(msg M) {
	log.Println("prepare Send")
	lxmsg, event, err := parseMsg(msg)
	if err != nil {
		return
	}
	send(lxmsg, event)
}

func Reply(msg M) {
	log.Println("prepare Reply")

	lxmsg, event, err := parseMsg(msg)
	if err != nil {
		return
	}
	reply(lxmsg, event)
}

func send(lxmsg *lxlib.LXMessage, event *linebot.Event) {
	message := linebot.NewTextMessage(lxmsg.Message.Text)

	log.Println("line-bot-sdk message:", message)
	res, err := bot.PushMessage(event.Source.RoomID, message).Do()
	if err != nil {
		log.Println("line-bot-sdk send error:", err)
	}
	log.Println("line-bot-sdk send result:", res)
}

func reply(lxmsg *lxlib.LXMessage, event *linebot.Event) {
	message := linebot.NewTextMessage(lxmsg.Message.Text)

	log.Println("line-bot-sdk message:", message)
	res, err := bot.ReplyMessage(event.ReplyToken, message).Do()
	if err != nil {
		log.Println("line-bot-sdk reply error:", err)
		log.Println("fallback to Send")
		send(lxmsg, event)
	}
	log.Println("line-bot-sdk reply result:", res)
}