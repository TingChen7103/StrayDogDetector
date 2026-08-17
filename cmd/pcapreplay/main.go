// pcapreplay: 離線重播 .pcapng 驗證解析管線
// 用法: pcapreplay <file1.pcapng> [file2.pcapng ...]
// 統計每個檔案解析出的框架、EntityAppear/EntitiesAppear 與角色名單
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"time"

	"gitlab.com/prilus/mabidilmeter/packet"
)

var logger = log.New(os.Stdout, "pcapreplay ", log.LstdFlags)

var pcRaceSet = map[uint32]bool{
	8001: true, 8002: true,
	9001: true, 9002: true,
	10001: true, 10002: true,
}

type entityRec struct {
	name    string
	raceId  uint32
	count   int
	isLocal bool
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: pcapreplay <file.pcapng> [...]")
		fmt.Println("       pcapreplay dump <file.pcapng> <entityId> [...]  (傾印指定實體的 appear 封包元素)")
		os.Exit(1)
	}

	if os.Args[1] == "dump" && len(os.Args) >= 4 {
		targets := map[uint64]bool{}
		for _, s := range os.Args[3:] {
			id, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				fmt.Println("bad id:", s)
				os.Exit(1)
			}
			targets[id] = true
		}
		dumpFile(os.Args[2], targets)
		return
	}

	grandEntries := 0
	grandPcNames := map[string]bool{}

	for _, file := range os.Args[1:] {
		entries, pcNames := replayFile(file)
		grandEntries += entries
		for n := range pcNames {
			grandPcNames[n] = true
		}
	}

	if len(os.Args) > 2 {
		names := make([]string, 0, len(grandPcNames))
		for n := range grandPcNames {
			names = append(names, n)
		}
		sort.Strings(names)

		fmt.Println("==== TOTAL ====")
		fmt.Println("total appear entries:", grandEntries)
		fmt.Println("unique PC names:", len(names))
		for _, n := range names {
			fmt.Println("  *", n)
		}
	}
}

func replayFile(file string) (int, map[string]bool) {
	fmt.Println("======================================")
	fmt.Println("FILE:", file)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx:      ctx,
		FileName: file,
	})
	if err != nil {
		logger.Println("open failed:", err)
		return 0, nil
	}
	defer r.Close()

	frames := 0
	opCount := map[uint32]int{}
	singleEntities := 0
	multiFrames := 0
	multiEntries := 0
	entityMap := map[uint64]*entityRec{}

	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()

loop:
	for {
		select {
		case p := <-r.PacketCh():
			timeout.Reset(3 * time.Second)
			frames++
			opCount[p.Op]++

			switch p.Op {
			case 0x520c:
				entity, err := packet.ParseEntityAppearPacket(p.Msg)
				if err != nil && (entity == nil || entity.Name == "" || entity.RaceId == 0) {
					continue
				}
				if entity == nil {
					continue
				}
				singleEntities++
				addEntity(entityMap, entity)

			case 0x5334:
				entities, err := packet.ParseEntitiesAppearPacket(p)
				if err != nil {
					logger.Println("ParseEntitiesAppearPacket failed:", err)
					continue
				}
				multiFrames++
				multiEntries += len(entities)
				for _, entity := range entities {
					addEntity(entityMap, entity)
				}
			}

		case <-timeout.C:
			break loop
		}
	}

	pcNames := map[string]bool{}
	names := []string(nil)
	for _, rec := range entityMap {
		if pcRaceSet[rec.raceId] && rec.name != "" && rec.name[0] != '_' {
			label := rec.name
			if rec.isLocal {
				label += " (local)"
			}
			names = append(names, fmt.Sprintf("%v race=%v seen=%v", label, rec.raceId, rec.count))
			pcNames[rec.name] = true
		}
	}
	sort.Strings(names)

	fmt.Println("frames:", frames)
	fmt.Println("0x520c singles parsed:", singleEntities, " 0x5334 frames:", multiFrames, " entries parsed:", multiEntries)
	fmt.Println("unique entities:", len(entityMap), " unique PC characters:", len(names))
	for _, n := range names {
		fmt.Println("  *", n)
	}

	topOps := make([]uint32, 0, len(opCount))
	for op := range opCount {
		topOps = append(topOps, op)
	}
	sort.Slice(topOps, func(i, j int) bool { return opCount[topOps[i]] > opCount[topOps[j]] })
	if len(topOps) > 8 {
		topOps = topOps[:8]
	}
	for _, op := range topOps {
		fmt.Printf("  op 0x%x: %v\n", op, opCount[op])
	}

	return singleEntities + multiEntries, pcNames
}

func truncStr(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "..."
	}
	return s
}

func dumpMsg(msg packet.Message) {
	for i, elem := range msg {
		fmt.Printf("  [%3d] type=%v  %v\n", i, elem.Type(), truncStr(elem.String(), 70))
	}
}

func dumpEntityMsg(source string, id uint64, msg packet.Message) {
	fmt.Println("======================================")
	fmt.Printf("%v  id=%v  elems=%v\n", source, id, len(msg))
	dumpMsg(msg)

	v, err := packet.ParseEntityAppearPacket(msg)
	fmt.Printf("--> ParseEntityAppearPacket: err=%v\n", err)
	if v != nil {
		fmt.Printf("--> parsed: Name=%q RaceId=%v OwnerId=%v IsLocalPC=%v equips=%v conds=%v guild=%q\n",
			v.Name, v.RaceId, v.OwnerId, v.IsLocalPC, len(v.EquipItemMap), len(v.CharacterConditionMap), v.GuildName)
	}
}

func dumpFile(file string, targets map[uint64]bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx:      ctx,
		FileName: file,
	})
	if err != nil {
		logger.Fatalln("open failed:", err)
	}
	defer r.Close()

	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case p := <-r.PacketCh():
			timeout.Reset(3 * time.Second)

			msgEntityId := func(m packet.Message) uint64 {
				if len(m) > 0 && m[0].Type() == packet.MessageElemTypeLong {
					return m[0].Data().(uint64)
				}
				return 0
			}

			switch p.Op {
			case 0x7530:
				// stat update private: 封包 id 應為本機玩家實體 id
				fmt.Printf("0x7530 statUpdatePrivate packet id=%v\n", p.Id)

			case 0x520c:
				if targets[p.Id] || targets[msgEntityId(p.Msg)] {
					dumpEntityMsg(fmt.Sprintf("0x520c single (packet id=%v)", p.Id), msgEntityId(p.Msg), p.Msg)
				}

			case 0x5334:
				// 走訪每個條目,傾印目標實體的子訊息
				msg := p.Msg
				if len(msg) < 1 {
					continue
				}
				msg = msg[1:]

				for len(msg) >= 3 {
					if msg[0].Type() != packet.MessageElemTypeShort ||
						msg[1].Type() != packet.MessageElemTypeInt ||
						msg[2].Type() != packet.MessageElemTypeBin {
						break
					}

					ttype, b := msg[0].Data().(uint16), msg[2].Data().([]byte)
					msg = msg[3:]

					if len(b) < 12 {
						continue
					}

					_, subId, subMsg, err := packet.GamePacketBodyReader(bytes.NewReader(b))
					if err != nil {
						continue
					}

					if targets[subId] || targets[msgEntityId(subMsg)] {
						dumpEntityMsg(fmt.Sprintf("0x5334 entry ttype=%v (blob id=%v)", ttype, subId), msgEntityId(subMsg), subMsg)
					}
				}
			}

		case <-timeout.C:
			return
		}
	}
}

func addEntity(m map[uint64]*entityRec, e *packet.EntityInfo) {
	rec := m[e.Id]
	if rec == nil {
		rec = &entityRec{}
		m[e.Id] = rec
	}

	rec.count++
	if e.Name != "" {
		rec.name = e.Name
	}
	if e.RaceId != 0 {
		rec.raceId = e.RaceId
	}
	if e.IsLocalPC {
		rec.isLocal = true
	}
}
