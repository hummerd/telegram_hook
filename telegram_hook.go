package telegram_hook

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	messageBuff = sync.Pool{New: func() interface{} { return &textBuffer{} }}
	requestBuff = sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}
)

// TelegramHook to send logs via the Telegram API.
type TelegramHook struct {
	AppName         string
	c               *http.Client
	authToken       string
	chatID          int
	apiEndpoint     string
	apiEndpointGet  string
	apiEndpointSend string
}

// apiRequest encapsulates the request structure we are sending to the
// Telegram API.
type apiRequest struct {
	ChatID    int                    `json:"chat_id"`
	Text      encoding.TextMarshaler `json:"text"`
	ParseMode string                 `json:"parse_mode,omitempty"`
}

// apiResponse encapsulates the response structure received from the
// Telegram API.
type apiResponse struct {
	Ok        bool         `json:"ok"`
	ErrorCode *int         `json:"error_code,omitempty"`
	Desc      *string      `json:"description,omitempty"`
	Result    *interface{} `json:"result,omitempty"`
}

type textBuffer struct {
	bytes.Buffer
}

func (tb *textBuffer) MarshalText() (text []byte, err error) {
	return tb.Bytes(), nil
}

// NewTelegramHook creates a new instance of a hook targeting the
// Telegram API.
func NewTelegramHook(appName, authToken string, chatID int) (*TelegramHook, error) {
	client := http.Client{}
	apiEndpoint := "https://api.telegram.org/bot" + authToken
	apiEndpointGet := apiEndpoint + "/getme"
	apiEndpointSend := apiEndpoint + "/sendmessage"

	h := TelegramHook{
		AppName:         appName,
		c:               &client,
		authToken:       authToken,
		chatID:          chatID,
		apiEndpoint:     apiEndpoint,
		apiEndpointGet:  apiEndpointGet,
		apiEndpointSend: apiEndpointSend,
	}

	// Verify the API token is valid and correct before continuing
	err := h.verifyToken()
	if err != nil {
		return nil, err
	}

	return &h, nil
}

// verifyToken issues a test request to the Telegram API to ensure the
// provided token is correct and valid.
func (hook *TelegramHook) verifyToken() error {
	res, err := hook.c.Get(hook.apiEndpointGet)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	apiRes := apiResponse{}
	err = json.NewDecoder(res.Body).Decode(&apiRes)
	if err != nil {
		return err
	}

	if !apiRes.Ok {
		// Received an error from the Telegram API
		msg := "Received error response from Telegram API"
		if apiRes.ErrorCode != nil {
			msg = fmt.Sprintf("%s (error code %d)", msg, *apiRes.ErrorCode)
		}
		if apiRes.Desc != nil {
			msg = fmt.Sprintf("%s: %s", msg, *apiRes.Desc)
		}
		j, _ := json.MarshalIndent(apiRes, "", "\t")
		msg = fmt.Sprintf("%s\n%s", msg, j)
		return fmt.Errorf(msg)
	}

	return nil
}

// sendMessage issues the provided message to the Telegram API.
func (hook *TelegramHook) sendMessage(msg *textBuffer) error {
	apiReq := apiRequest{
		ChatID:    hook.chatID,
		Text:      msg,
		ParseMode: "HTML",
	}

	buff := requestBuff.Get().(*bytes.Buffer)
	buff.Reset()
	defer requestBuff.Put(buff)

	err := json.NewEncoder(buff).Encode(apiReq)
	if err != nil {
		return err
	}

	res, err := hook.c.Post(hook.apiEndpointSend, "application/json", buff)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error when issuing request to Telegram API, %v", err)
		return err
	}
	defer res.Body.Close()

	apiRes := apiResponse{}
	err = json.NewDecoder(res.Body).Decode(&apiRes)
	if err != nil {
		return err
	}

	if !apiRes.Ok {
		// Received an error from the Telegram API
		msg := "Received error response from Telegram API"
		if apiRes.ErrorCode != nil {
			msg = fmt.Sprintf("%s (error code %d)", msg, *apiRes.ErrorCode)
		}
		if apiRes.Desc != nil {
			msg = fmt.Sprintf("%s: %s", msg, *apiRes.Desc)
		}
		return fmt.Errorf(msg)
	}

	return nil
}

// createMessage crafts an HTML-formatted message to send to the
// Telegram API.
func (hook *TelegramHook) writeMessage(entry *logrus.Entry, buff *textBuffer) {
	switch entry.Level {
	case logrus.PanicLevel:
		buff.WriteString("<b>PANIC</b>")
	case logrus.FatalLevel:
		buff.WriteString("<b>FATAL</b>")
	case logrus.ErrorLevel:
		buff.WriteString("<b>ERROR</b>")
	}

	buff.WriteString("@")
	buff.WriteString(hook.AppName)
	buff.WriteString(" - ")
	buff.WriteString(entry.Message)

	errToLogI := entry.Data[logrus.ErrorKey]
	if errToLogI != nil {
		errToLog, ok := errToLogI.(error)
		if ok && errToLog != nil {
			buff.WriteString(": ")
			buff.WriteString(errToLog.Error())
		}
	}

	buff.WriteString("\n<pre>\n")
	enc := json.NewEncoder(buff)
	enc.SetIndent("", "\t")
	enc.Encode(entry.Data)
	buff.WriteString("\n</pre>")
}

// Fire emits a log message to the Telegram API.
func (hook *TelegramHook) Fire(entry *logrus.Entry) error {
	buff := messageBuff.Get().(*textBuffer)
	buff.Reset()
	defer messageBuff.Put(buff)

	hook.writeMessage(entry, buff)
	err := hook.sendMessage(buff)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to send message, %v", err)
		return err
	}

	return nil
}

// Levels returns the log levels that the hook should be enabled for.
func (hook *TelegramHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}
