package app

import (
	"encoding/binary"
	"path/filepath"
	"testing"
	"time"

	"blonymonitorv2/internal/farmstats"
	"blonymonitorv2/internal/packet"
)

func lifeCoinItem(instance uint64, amount uint16) packet.Message {
	data := make([]byte, 80)
	binary.LittleEndian.PutUint32(data[0:], inventoryCurrencyPocket)
	binary.LittleEndian.PutUint32(data[4:], lifeAssociationCoinItemID)
	binary.LittleEndian.PutUint16(data[36:], amount)
	return packet.Message{
		packet.NewMessageElemByte(0), packet.NewMessageElemLong(101), packet.NewMessageElemByte(0),
		packet.NewMessageElemString("Tester"), packet.NewMessageElemString(""), packet.NewMessageElemString(""), packet.NewMessageElemInt(1),
		packet.NewMessageElemLong(instance), packet.NewMessageElemByte(2), packet.NewMessageElemBin(data),
	}
}

func lifeCoinSnapshot(amounts ...uint16) packet.Message {
	msg := lifeCoinItem(1, amounts[0])
	for index, amount := range amounts[1:] {
		data := make([]byte, 80)
		binary.LittleEndian.PutUint32(data[0:], inventoryCurrencyPocket)
		binary.LittleEndian.PutUint32(data[4:], lifeAssociationCoinItemID)
		binary.LittleEndian.PutUint16(data[36:], amount)
		msg = append(msg,
			packet.NewMessageElemLong(uint64(index+2)),
			packet.NewMessageElemByte(2),
			packet.NewMessageElemBin(data),
		)
	}
	return msg
}

func TestLifeCoinSnapshotUsesInnerCharacterIDAndAggregatesStacks(t *testing.T) {
	store, err := farmstats.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := NewApp()
	a.farmStatsStore = store
	at := time.Date(2026, 9, 4, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	// Login/channel responses use a synthetic outer entity ID. The character
	// header inside the message remains the owner of the inventory snapshot.
	a.handleLifeCoinPacket(&packet.GamePacket{
		At: at, Op: opcodeCharacterData, Id: 1152921504606846977,
		Msg: lifeCoinSnapshot(50000, 46300, 50000, 33595),
	})
	u, err := store.CurrencyUsage("character:101", lifeAssociationCurrency)
	if err != nil {
		t.Fatal(err)
	}
	if !u.BalanceKnown || u.Balance != 179895 {
		t.Fatalf("usage = %#v", u)
	}
	outer, err := store.CurrencyUsage("character:1152921504606846977", lifeAssociationCurrency)
	if err != nil {
		t.Fatal(err)
	}
	if outer.BalanceKnown {
		t.Fatalf("synthetic outer entity was used as owner: %#v", outer)
	}
}

func TestLifeCoinSnapshotAndStackDeltaAreCharacterScoped(t *testing.T) {
	store, err := farmstats.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := NewApp()
	a.farmStatsStore = store
	at := time.Date(2026, 9, 4, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	a.handleLifeCoinPacket(&packet.GamePacket{At: at, Op: opcodeCharacterData, Id: 101, Msg: lifeCoinItem(1, 30000)})
	a.handleLifeCoinPacket(&packet.GamePacket{At: at.Add(time.Minute), Op: opcodeItemStackUpdate, Id: 101, Msg: packet.Message{packet.NewMessageElemLong(1), packet.NewMessageElemShort(30200), packet.NewMessageElemInt(inventoryCurrencyPocket)}})
	a.handleLifeCoinPacket(&packet.GamePacket{At: at, Op: opcodeItemStackUpdate, Id: 202, Msg: packet.Message{packet.NewMessageElemLong(1), packet.NewMessageElemShort(35000), packet.NewMessageElemInt(inventoryCurrencyPocket)}})
	a.flushPendingLifeCoinBalance("character:101", 0)
	u, err := store.CurrencyUsage("character:101", lifeAssociationCurrency)
	if err != nil {
		t.Fatal(err)
	}
	if u.Added != 200 || u.Spent != 0 || u.Balance != 30200 {
		t.Fatalf("usage = %#v", u)
	}
	other, err := store.CurrencyUsage("character:202", lifeAssociationCurrency)
	if err != nil {
		t.Fatal(err)
	}
	if other.Added != 0 || other.Spent != 0 {
		t.Fatalf("other character was changed: %#v", other)
	}
}

func TestLifeCoinStackRebalanceRecordsOnlyNetBalanceChange(t *testing.T) {
	store, err := farmstats.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := NewApp()
	a.farmStatsStore = store
	at := time.Date(2026, 9, 5, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	a.handleLifeCoinPacket(&packet.GamePacket{
		At: at, Op: opcodeCharacterData, Id: 101,
		Msg: lifeCoinSnapshot(50000, 50000, 587, 50000, 50000),
	})

	a.handleLifeCoinPacket(&packet.GamePacket{
		At: at.Add(time.Minute), Op: opcodeItemStackUpdate, Id: 101,
		Msg: packet.Message{packet.NewMessageElemLong(3), packet.NewMessageElemShort(50000), packet.NewMessageElemInt(inventoryCurrencyPocket)},
	})
	intermediate, err := store.CurrencyUsage("character:101", lifeAssociationCurrency)
	if err != nil {
		t.Fatal(err)
	}
	if intermediate.Added != 0 || intermediate.Spent != 0 || intermediate.Balance != 200587 {
		t.Fatalf("intermediate stack update was persisted: %#v", intermediate)
	}
	a.handleLifeCoinPacket(&packet.GamePacket{
		At: at.Add(time.Minute + 10*time.Millisecond), Op: opcodeItemStackUpdate, Id: 101,
		Msg: packet.Message{packet.NewMessageElemLong(4), packet.NewMessageElemShort(3148), packet.NewMessageElemInt(inventoryCurrencyPocket)},
	})
	a.flushPendingLifeCoinBalance("character:101", 0)

	usage, err := store.CurrencyUsage("character:101", lifeAssociationCurrency)
	if err != nil {
		t.Fatal(err)
	}
	if usage.Added != 2561 || usage.Spent != 0 || usage.Balance != 203148 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestLifeCoinStackDeltaSettlesAfterQuietPeriod(t *testing.T) {
	store, err := farmstats.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := NewApp()
	a.farmStatsStore = store
	at := time.Date(2026, 9, 5, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	a.handleLifeCoinPacket(&packet.GamePacket{At: at, Op: opcodeCharacterData, Id: 101, Msg: lifeCoinItem(1, 30000)})
	a.handleLifeCoinPacket(&packet.GamePacket{
		At: at.Add(time.Minute), Op: opcodeItemStackUpdate, Id: 101,
		Msg: packet.Message{packet.NewMessageElemLong(1), packet.NewMessageElemShort(30200), packet.NewMessageElemInt(inventoryCurrencyPocket)},
	})

	deadline := time.Now().Add(2 * time.Second)
	for {
		usage, err := store.CurrencyUsage("character:101", lifeAssociationCurrency)
		if err != nil {
			t.Fatal(err)
		}
		if usage.Added == 200 && usage.Balance == 30200 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pending balance was not settled: %#v", usage)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLifeCoinAuthoritativeSnapshotCancelsPendingStackBalance(t *testing.T) {
	store, err := farmstats.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := NewApp()
	a.farmStatsStore = store
	at := time.Date(2026, 9, 5, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	a.handleLifeCoinPacket(&packet.GamePacket{At: at, Op: opcodeCharacterData, Id: 101, Msg: lifeCoinItem(1, 30000)})
	a.handleLifeCoinPacket(&packet.GamePacket{
		At: at.Add(time.Minute), Op: opcodeItemStackUpdate, Id: 101,
		Msg: packet.Message{packet.NewMessageElemLong(1), packet.NewMessageElemShort(50000), packet.NewMessageElemInt(inventoryCurrencyPocket)},
	})
	a.handleLifeCoinPacket(&packet.GamePacket{
		At: at.Add(time.Minute + 10*time.Millisecond), Op: opcodeCharacterData, Id: 101,
		Msg: lifeCoinItem(1, 31000),
	})
	a.flushPendingLifeCoinBalance("character:101", 0)

	usage, err := store.CurrencyUsage("character:101", lifeAssociationCurrency)
	if err != nil {
		t.Fatal(err)
	}
	if usage.Added != 1000 || usage.Spent != 0 || usage.Balance != 31000 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestLifeCoinAuthoritativeSyncInfersMissedChange(t *testing.T) {
	store, err := farmstats.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := NewApp()
	a.farmStatsStore = store
	at := time.Date(2026, 9, 4, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	a.handleLifeCoinPacket(&packet.GamePacket{At: at, Op: opcodeCharacterData, Id: 101, Msg: lifeCoinItem(1, 30000)})
	a.handleLifeCoinPacket(&packet.GamePacket{At: at.Add(time.Minute), Op: opcodeCharacterData, Id: 101, Msg: lifeCoinItem(1, 32000)})
	u, err := store.CurrencyUsage("character:101", lifeAssociationCurrency)
	if err != nil {
		t.Fatal(err)
	}
	if u.Added != 2000 || u.Balance != 32000 {
		t.Fatalf("usage = %#v", u)
	}
}

func TestLifeCoinTracksCaptureStartedMidSession(t *testing.T) {
	store, err := farmstats.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := NewApp()
	a.farmStatsStore = store
	at := time.Date(2026, 9, 4, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	a.handleLifeCoinPacket(&packet.GamePacket{At: at, Op: opcodeItemStackUpdate, Id: 101, Msg: packet.Message{packet.NewMessageElemLong(1), packet.NewMessageElemShort(30295), packet.NewMessageElemInt(inventoryCurrencyPocket)}})
	a.handleLifeCoinPacket(&packet.GamePacket{At: at.Add(time.Minute), Op: opcodeItemStackUpdate, Id: 101, Msg: packet.Message{packet.NewMessageElemLong(1), packet.NewMessageElemShort(32295), packet.NewMessageElemInt(inventoryCurrencyPocket)}})
	u, err := store.CurrencyUsage("character:101", lifeAssociationCurrency)
	if err != nil {
		t.Fatal(err)
	}
	if u.Added != 0 || u.Spent != 0 || u.BalanceKnown {
		t.Fatalf("usage = %#v", u)
	}
}

func TestLifeCoinTextBalanceSync(t *testing.T) {
	store, err := farmstats.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := NewApp()
	a.farmStatsStore = store
	at := time.Date(2026, 9, 4, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	a.handleLifeCoinPacket(&packet.GamePacket{At: at, Op: opcodeServerTimeSync, Id: 101, Msg: packet.Message{packet.NewMessageElemString("*需提交的生活协会硬币：5000 / 183595个")}})
	u, err := store.CurrencyUsage("character:101", lifeAssociationCurrency)
	if err != nil {
		t.Fatal(err)
	}
	if !u.BalanceKnown || u.Balance != 183595 {
		t.Fatalf("usage = %#v", u)
	}
}

func TestLifeCoinEmptySnapshotDoesNotZeroKnownBalance(t *testing.T) {
	store, err := farmstats.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := NewApp()
	a.farmStatsStore = store
	at := time.Date(2026, 9, 9, 16, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	a.handleLifeCoinPacket(&packet.GamePacket{At: at, Op: opcodeCharacterData, Id: 101, Msg: lifeCoinItem(1, 30000)})
	emptySnapshot := packet.Message{
		packet.NewMessageElemByte(0), packet.NewMessageElemLong(101), packet.NewMessageElemByte(0),
		packet.NewMessageElemString("Tester"), packet.NewMessageElemString(""), packet.NewMessageElemString(""), packet.NewMessageElemInt(1),
	}
	a.handleLifeCoinPacket(&packet.GamePacket{At: at.Add(time.Minute), Op: opcodeCharacterData, Id: 101, Msg: emptySnapshot})
	u, err := store.CurrencyUsage("character:101", lifeAssociationCurrency)
	if err != nil {
		t.Fatal(err)
	}
	if u.Added != 0 || u.Spent != 0 || u.Balance != 30000 {
		t.Fatalf("unparsed snapshot zeroed the wallet: %#v", u)
	}
}

func TestLifeCoinSnapshotWithOtherItemsCanSyncZero(t *testing.T) {
	store, err := farmstats.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := NewApp()
	a.farmStatsStore = store
	at := time.Date(2026, 9, 9, 16, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	a.handleLifeCoinPacket(&packet.GamePacket{At: at, Op: opcodeCharacterData, Id: 101, Msg: lifeCoinItem(1, 30000)})
	other := make([]byte, 80)
	binary.LittleEndian.PutUint32(other[0:], 2)
	binary.LittleEndian.PutUint32(other[4:], 5100428)
	binary.LittleEndian.PutUint16(other[36:], 60)
	a.handleLifeCoinPacket(&packet.GamePacket{
		At: at.Add(time.Minute), Op: opcodeCharacterData, Id: 101,
		Msg: packet.Message{
			packet.NewMessageElemByte(0), packet.NewMessageElemLong(101), packet.NewMessageElemByte(0),
			packet.NewMessageElemString("Tester"), packet.NewMessageElemString(""), packet.NewMessageElemString(""), packet.NewMessageElemInt(1),
			packet.NewMessageElemLong(9), packet.NewMessageElemByte(2), packet.NewMessageElemBin(other),
		},
	})
	u, err := store.CurrencyUsage("character:101", lifeAssociationCurrency)
	if err != nil {
		t.Fatal(err)
	}
	if u.Added != 0 || u.Spent != 30000 || u.Balance != 0 {
		t.Fatalf("parsed snapshot without coins should sync zero: %#v", u)
	}
}
