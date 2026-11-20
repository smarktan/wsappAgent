package database

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func newTestMessageStore(t *testing.T) *MessageStore {
	t.Helper()

	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := createTables(db); err != nil {
		t.Fatalf("create tables: %v", err)
	}
	if err := runMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	return &MessageStore{db: db}
}

func TestStoreChatKeepsNewestTimestamp(t *testing.T) {
	store := newTestMessageStore(t)
	older := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	newer := older.Add(2 * time.Hour)

	if err := store.StoreChat("123@s.whatsapp.net", "Alice", newer); err != nil {
		t.Fatalf("store newer chat: %v", err)
	}
	if err := store.StoreChat("123@s.whatsapp.net", "Alice Old", older); err != nil {
		t.Fatalf("store older chat: %v", err)
	}

	var name string
	var got time.Time
	if err := store.db.QueryRow("SELECT name, last_message_time FROM chats WHERE jid = ?", "123@s.whatsapp.net").Scan(&name, &got); err != nil {
		t.Fatalf("read chat: %v", err)
	}
	if !got.Equal(newer) {
		t.Fatalf("last_message_time = %v, want %v", got, newer)
	}
	if name != "Alice Old" {
		t.Fatalf("name = %q, want latest non-empty name", name)
	}
}

func TestStoreChatEmptyNameDoesNotEraseExistingName(t *testing.T) {
	store := newTestMessageStore(t)
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)

	if err := store.StoreChat("123@s.whatsapp.net", "Alice", now); err != nil {
		t.Fatalf("store named chat: %v", err)
	}
	if err := store.StoreChat("123@s.whatsapp.net", "", now.Add(time.Hour)); err != nil {
		t.Fatalf("store empty-name chat: %v", err)
	}

	var name string
	if err := store.db.QueryRow("SELECT name FROM chats WHERE jid = ?", "123@s.whatsapp.net").Scan(&name); err != nil {
		t.Fatalf("read chat: %v", err)
	}
	if name != "Alice" {
		t.Fatalf("name = %q, want existing name", name)
	}
}

func TestLatestIncomingPushNameIgnoresRawSenderIDs(t *testing.T) {
	store := newTestMessageStore(t)
	now := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)
	chat := "199592163471563@lid"

	if err := store.StoreMessage("1", chat, "8613800138000", "8613800138000", "hi", now, false, "", "", "", "", nil, nil, nil, 0); err != nil {
		t.Fatalf("store raw id message: %v", err)
	}
	if err := store.StoreMessage("2", chat, "8618627915687", "Me", "44", now.Add(time.Minute), true, "", "", "", "", nil, nil, nil, 0); err != nil {
		t.Fatalf("store own message: %v", err)
	}
	if err := store.StoreMessage("3", chat, "8613800138000", "Ada", "hello", now.Add(2*time.Minute), false, "", "", "", "", nil, nil, nil, 0); err != nil {
		t.Fatalf("store named message: %v", err)
	}

	got, err := store.LatestIncomingPushName(chat)
	if err != nil {
		t.Fatalf("LatestIncomingPushName: %v", err)
	}
	if got != "Ada" {
		t.Fatalf("LatestIncomingPushName() = %q, want Ada", got)
	}
}

func TestRenameChatReplacesTheStoredGroupName(t *testing.T) {
	store := newTestMessageStore(t)
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

	if err := store.StoreChat("120363000000000001@g.us", "Old Group Name", now); err != nil {
		t.Fatalf("store chat: %v", err)
	}
	if err := store.RenameChat("120363000000000001@g.us", "New Group Name"); err != nil {
		t.Fatalf("rename chat: %v", err)
	}

	var name string
	var got time.Time
	if err := store.db.QueryRow("SELECT name, last_message_time FROM chats WHERE jid = ?", "120363000000000001@g.us").Scan(&name, &got); err != nil {
		t.Fatalf("read chat: %v", err)
	}
	if name != "New Group Name" {
		t.Fatalf("name = %q, want the new group name", name)
	}
	if !got.Equal(now) {
		t.Fatalf("last_message_time = %v, want it unchanged at %v", got, now)
	}
}

func TestRenameChatOfAnUnknownGroupCreatesNoChat(t *testing.T) {
	store := newTestMessageStore(t)

	if err := store.RenameChat("999@g.us", "Never Seen"); err != nil {
		t.Fatalf("rename chat: %v", err)
	}

	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM chats").Scan(&count); err != nil {
		t.Fatalf("count chats: %v", err)
	}
	if count != 0 {
		t.Fatalf("chats = %d, want 0", count)
	}
}
