package whatsapp

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	waWeb "go.mau.fi/whatsmeow/proto/waWeb"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func TestLatestHistoryMessageTimeUsesMaxTimestamp(t *testing.T) {
	older := uint64(1_768_000_000)
	newer := older + 3600

	got, ok := latestHistoryMessageTime([]*waHistorySync.HistorySyncMsg{
		historyMsg("old", older, false),
		nil,
		historyMsg("new", newer, false),
	})
	if !ok {
		t.Fatal("latestHistoryMessageTime() ok = false, want true")
	}

	want := time.Unix(int64(newer), 0)
	if !got.Equal(want) {
		t.Fatalf("latestHistoryMessageTime() = %v, want %v", got, want)
	}
}

func TestHistoryMessageSenderUsesFullOwnJID(t *testing.T) {
	own := types.JID{User: "111", Server: "s.whatsapp.net"}
	client := &Client{
		Client: &whatsmeow.Client{Store: &store.Device{ID: &own}},
	}
	chat := types.JID{User: "222", Server: "s.whatsapp.net"}

	sender, fromMe := client.historyMessageSender(chat, &waCommon.MessageKey{FromMe: proto.Bool(true)})
	if !fromMe {
		t.Fatal("fromMe = false, want true")
	}
	if sender != "111@s.whatsapp.net" {
		t.Fatalf("sender = %q, want full own JID", sender)
	}
}

func TestHistoryMessageSenderPrefersParticipant(t *testing.T) {
	client := &Client{}
	chat := types.JID{User: "12345", Server: "g.us"}

	sender, fromMe := client.historyMessageSender(chat, &waCommon.MessageKey{
		FromMe:      proto.Bool(false),
		Participant: proto.String("333@s.whatsapp.net"),
	})
	if fromMe {
		t.Fatal("fromMe = true, want false")
	}
	if sender != "333@s.whatsapp.net" {
		t.Fatalf("sender = %q, want participant JID", sender)
	}
}

func TestIndividualChatNamePrefersSavedNameOverLID(t *testing.T) {
	got := individualChatName("Ada", "8613800138000", "199592163471563", "8618627915687", "8618627915687")
	if got != "Ada" {
		t.Fatalf("individualChatName() = %q, want Ada", got)
	}
}

func TestIndividualChatNameUsesPhoneNumberInsteadOfLID(t *testing.T) {
	got := individualChatName("", "8613800138000", "199592163471563", "8618627915687", "8618627915687")
	if got != "8613800138000" {
		t.Fatalf("individualChatName() = %q, want the phone number", got)
	}
}

func TestIndividualChatNameDoesNotUseOwnNumber(t *testing.T) {
	got := individualChatName("", "", "199592163471563", "8618627915687", "8618627915687")
	if got != "199592163471563" {
		t.Fatalf("individualChatName() = %q, want the chat user", got)
	}
}

func TestPreferPushNameOnlyForTheOtherParty(t *testing.T) {
	if got := preferPushName("199592163471563", "Ada", false); got != "Ada" {
		t.Fatalf("preferPushName(other) = %q, want Ada", got)
	}
	if got := preferPushName("199592163471563", "Me", true); got != "199592163471563" {
		t.Fatalf("preferPushName(self) = %q, want the current placeholder", got)
	}
	if got := preferPushName("Ada", "Someone Else", false); got != "Ada" {
		t.Fatalf("preferPushName(saved) = %q, want Ada", got)
	}
}

func TestShouldReplaceLIDPlaceholder(t *testing.T) {
	jid := "199592163471563@lid"
	if !shouldReplaceStoredChatName(jid, "199592163471563", "Ada") {
		t.Fatal("expected a real name to replace the LID placeholder")
	}
	if !shouldReplaceStoredChatName(jid, "199592163471563", "8613800138000") {
		t.Fatal("expected a resolved phone number to replace the LID placeholder")
	}
	if shouldReplaceStoredChatName(jid, "Ada", "8613800138000") {
		t.Fatal("a phone number must not replace a saved name")
	}
	if shouldReplaceStoredChatName("8613800138000@s.whatsapp.net", "8613800138000", "8613900139000") {
		t.Fatal("an existing phone-number title must stay until a real name is known")
	}
}

func TestHistoryCounterpartPushNameSkipsOwnMessages(t *testing.T) {
	own := historyMsg("me", 1, true)
	own.Message.PushName = proto.String("Me")
	theirs := historyMsg("them", 2, false)
	theirs.Message.PushName = proto.String("Ada")

	got := historyCounterpartPushName([]*waHistorySync.HistorySyncMsg{own, theirs})
	if got != "Ada" {
		t.Fatalf("historyCounterpartPushName() = %q, want Ada", got)
	}
}

func historyMsg(id string, timestamp uint64, fromMe bool) *waHistorySync.HistorySyncMsg {
	return &waHistorySync.HistorySyncMsg{
		Message: &waWeb.WebMessageInfo{
			Key: &waCommon.MessageKey{
				ID:     proto.String(id),
				FromMe: proto.Bool(fromMe),
			},
			MessageTimestamp: proto.Uint64(timestamp),
			Message: &waE2E.Message{
				Conversation: proto.String(id),
			},
		},
	}
}
