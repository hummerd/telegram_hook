package telegram_hook

import (
	"encoding/json"
	"os"
	"testing"
)

func TestTextBuffer(t *testing.T) {
	buff := &textBuffer{}
	buff.WriteString(`str"<>` + "\n\t")

	s := struct {
		Name string      `json:"name"`
		Str  *textBuffer `json:"str"`
	}{
		Name: `str"<>` + "\n\t",
		Str:  buff,
	}

	b, err := json.Marshal(s)
	if err != nil {
		t.Error(err)
	}

	want := `{"name": "str\"\u003c\u003e\n\t","str": "str\"\u003c\u003e\n\t"}`
	if string(b) != want {
		t.Error("Want", want, "get", string(b))
	}
}

func TestNewHook(t *testing.T) {
	_, err := NewTelegramHook("", "", "")
	if err == nil {
		t.Errorf("No error on invalid Telegram API token.")
	}

	_, err = NewTelegramHook("", os.Getenv("TELEGRAM_TOKEN"), "")
	if err != nil {
		t.Fatalf("Error on valid Telegram API token: %s", err)
	}

	h, _ := NewTelegramHook("testing", os.Getenv("TELEGRAM_TOKEN"), os.Getenv("TELEGRAM_TARGET"))
	if err != nil {
		t.Fatalf("Error on valid Telegram API token and target: %s", err)
	}
	log.AddHook(h)

	log.WithFields(log.Fields{
		"animal": "walrus",
		"number": 1,
		"size":   10,
	}).Errorf("A walrus appears")
}
