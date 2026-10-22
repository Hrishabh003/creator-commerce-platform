package ai

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestCategorize(t *testing.T) {
	cases := map[string]string{
		"I was charged twice, please refund me":                "billing",
		"My package has not arrived, tracking shows late":      "shipping",
		"The sticker I received is damaged":                    "returns",
		"Hello there":                                          "general",
		"I forgot my password and cannot login to my account": "account",
	}
	for in, want := range cases {
		if got := Categorize(in); got != want {
			t.Errorf("Categorize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKeywords(t *testing.T) {
	got := Keywords("Where is my package? Tracking shows nothing")
	want := []string{"package", "tracking", "shows", "nothing"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Keywords = %v, want %v", got, want)
	}
}

func TestTemplateDrafterWithAndWithoutSources(t *testing.T) {
	d := TemplateDrafter{}
	with, _ := d.Draft(context.Background(), DraftInput{
		CustomerName: "Maya Chen", Category: "shipping",
		Articles: []Article{{Title: "Shipping times", Body: "Standard shipping takes 4 to 7 days. Track it by email."}},
	})
	if !strings.Contains(with, "Hi Maya") || !strings.Contains(with, "Standard shipping takes 4 to 7 days.") {
		t.Errorf("unexpected draft: %q", with)
	}
	without, _ := d.Draft(context.Background(), DraftInput{CustomerName: "Maya Chen"})
	if !strings.Contains(without, "teammate") {
		t.Errorf("expected hand-off wording, got %q", without)
	}
}
