package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"sync"

	"gitlab.com/prilus/mabidilmeter/packet"
)

type eventPublisher struct {
	sync.Mutex

	// non-mutable
	ctx         context.Context
	r           *packet.GameServerPacketReader
	clientMap   map[uint32]*eventClient
	entityCache entityCache

	// mutable
	currentClientId uint32
	localPcId       uint64 // 本機玩家實體 id (由 0x7530 statUpdatePrivate 判定)
}

// 人偶版型的主人 id 位於延伸區塊,以候選 Long 對照快取中的已知玩家 id 解析
// 必須在持有 t.Lock 時呼叫
func (t *eventPublisher) resolveMarionetteOwner(entity *packet.EntityInfo) {
	if entity.OwnerId != 0 || !marionetteRaceSet[entity.RaceId] {
		return
	}

	matched := []uint64(nil)
	for _, cand := range entity.OwnerCandidates {
		e := t.entityCache[cand]
		if e == nil || !e.IsUser() {
			continue
		}

		dup := false
		for _, m := range matched {
			if m == cand {
				dup = true
				break
			}
		}

		if !dup {
			matched = append(matched, cand)
		}
	}

	if len(matched) < 1 {
		return
	}

	entity.OwnerId = matched[0]

	if len(matched) > 1 {
		// 延伸區塊出現多個玩家 id: 單人實測只會有主人一個,此警告代表隊伍情境需要人工確認
		logger.Println("marionette owner ambiguous:", entity.Id, "candidates:", matched, "chose:", matched[0])
	}
}

type eventClient struct {
	ctx context.Context
	ch  chan<- iEvent
}

const (
	opcodeEntityAppear      = 0x520c
	opcodeEntityDisappear   = 0x520d
	OpcodeCreatureBodyUpdate = 0x520e
	opcodeEntitiesAppear    = 0x5334
	opcodeEntitiesDisappear = 0x5335
	opcodeEquipmentChanged  = 0x59e6
	opcodeUnequipment       = 0x59e7
	opcodeStatUpdatePrivate = 0x7530
	opcodeSetFinisher       = 0x7921
	opcodeCombatAction      = 0x7926
	opcodeEffectDelayed     = 0x9095
	opcodeConditionUpdate   = 0xa028
)

/*
	延遲傷害 (op 0x9095) 的效果類型 id。

	連續攻擊、星塵等技能的傷害不走一般的 CombatAction,而是由這個
	「延遲效果」封包送出,靠 Msg[1] 的類型 id 辨識。

	這個 id 是遊戲資料表中的「位置索引」而非固定代號,只要改版在它前面
	插入新項目,整串就會往後位移一格 —— 這就是為什麼它每次改版都會變
	(318 -> 319 正是插入一筆的典型 +1 位移)。

	因此這裡接受一組已知 id 而非單一值: 保留舊值可讓過去錄製的 pcapng /
	ndjson 仍能正確重播,新值則對應改版後的封包。
	若下次改版又位移,程式會在 log 印出 "unknown delayed-damage ttype",
	直接把新的號碼告訴你,補進這個 map 即可。
*/
var delayedDamageTypeSet = map[uint32]bool{
	318: true, // 2026-08 改版前
	319: true, // 2026-08 改版後
}

/*
	延遲傷害封包的訊息結構: Int(delay), Int(type), Int(damage), Byte,
	Int, Long(attackerId), Short(skillId)。

	實測 3 份擷取檔中所有 op 0x9095 的類型,只有延遲傷害是這個結構,
	其餘型別的結構都不同,因此可用來辨識「這其實是延遲傷害,只是 id 變了」。
*/
func isDelayedDamageShape(msg packet.Message) bool {
	if len(msg) < 7 {
		return false
	}

	want := []packet.MessageElemType{
		packet.MessageElemTypeInt,
		packet.MessageElemTypeInt,
		packet.MessageElemTypeInt,
		packet.MessageElemTypeByte,
		packet.MessageElemTypeInt,
		packet.MessageElemTypeLong,
		packet.MessageElemTypeShort,
	}

	for i, t := range want {
		if msg[i].Type() != t {
			return false
		}
	}

	return true
}

var warnedDelayedTypes = map[uint32]bool{}

// 同一個未知 id 只警告一次,避免洗版
func warnUnknownDelayedType(ttype uint32) {
	if warnedDelayedTypes[ttype] {
		return
	}
	warnedDelayedTypes[ttype] = true

	logger.Printf("unknown delayed-damage ttype %d -- 看起來是連續攻擊/星塵的延遲傷害封包,"+
		"但類型 id 不在已知清單中 (可能又改版位移了)。"+
		"請把 %d 加進 eventPublisher.go 的 delayedDamageTypeSet", ttype, ttype)
}

/*
	需要保留「效果參數字串」的條件 id。

	條件封包的參數字串帶著該次施放的實際加成數值 (例如戰場的序曲的
	最大攻擊力%、活潑板的魔法攻擊力%)。實測 94.4% 的條件都帶參數,
	全部輸出會讓 ndjson 多出約 15% 體積,因此只保留真的要顯示數值的條件。

	前端的顯示欄位設定在 front/src/conditionWhitelist.ts 的 musicBuffDisplay,
	要新增條件時兩邊都要加。
*/
var conditionParamCCIds = map[uint32]bool{
	680: true, // 戰場的序曲
	192: true, // 活潑板
	193: true, // 進行曲
}

// 只有白名單內的條件才輸出參數字串,其餘回傳空字串
func conditionParams(ccId uint32, params string) string {
	if !conditionParamCCIds[ccId] {
		return ""
	}
	return params
}

// 人偶召喚物種族白名單: 幕演出實體(990104)、皮埃羅(990125)、巨靈(990204)、傀儡實體(990213)
// 990104 是「第X幕」系技能實際造成傷害的臨時召喚實體 (2026-07-29 兩人隊實測確認)
var marionetteRaceSet = map[uint32]bool{
	990104: true,
	990125: true,
	990204: true,
	990213: true,
}

var le = binary.LittleEndian

func newEventPublisher(ctx context.Context, r *packet.GameServerPacketReader) *eventPublisher {
	v := &eventPublisher{
		ctx:         ctx,
		r:           r,
		clientMap:   make(map[uint32]*eventClient),
		entityCache: make(entityCache),

		currentClientId: 1,
	}

	go v.loop()

	return v
}

func (t *eventPublisher) loop() {
	debug := false

	for {
		select {
		case <-t.ctx.Done():
			return
		case p := <-t.r.PacketCh():

			if debug {
				logger.Printf("packet op %x id %x", p.Op, p.Id)
				for i, msg := range p.Msg {
					logger.Println("* msg", i, msg.Type(), msg.String())
				}
			}

			switch p.Op {

			// short packet
			case 0:
				continue

			case opcodeEntityAppear:
				entity, err := packet.ParseEntityAppearPacket(p.Msg)
				partial := err != nil
				if partial {
					logger.Println("ParseEntityAppearPacket failed:", err)
					// 與複數路徑一致: 名字與種族已解析出來就保留部分資訊
					if entity == nil || entity.Name == "" || entity.RaceId == 0 {
						continue
					}
				}

				if entity == nil || len(entity.Name) <= 0 || entity.Name[0] == '_' {
					// ignore npc or nil
					continue
				}

				if entity.Id == t.localPcId {
					entity.IsLocalPC = true
				}

				t.Lock()
				if partial {
					// 部分實體: 以快取補齊零值欄位,不可覆蓋已知的完整資訊
					t.entityCache.mergeFromCache(entity)
				}
				t.entityCache.add(entity, p.At)
				t.resolveMarionetteOwner(entity)
				t.Unlock()

				e := toEventEntityAppear(p.At.Unix(), entity)

				t.publish(e)

				if partial {
					// 解析中斷的裝備/狀態清單不可信,跳過差異比對 (避免偽卸裝事件)
					continue
				}

				for _, v := range entity.CharacterConditionMap {
					if !t.entityCache.addOrUpdateCondition(entity.Id, v) {
						continue
					}

					attackerId := ""
					if v.AttackerId != 0 {
						attackerId = strconv.FormatUint(v.AttackerId, 10)
					}

					e := &eventCharacterConditionEnable{
						eventBase: eventBase{
							EventId: eventIdCharacterConditionEnable,
							At:      p.At.Unix(),
							Id:      strconv.FormatUint(entity.Id, 10),
						},
						CCId:       v.CCId,
						DisableAt:  v.DisableAt,
						AttackerId: attackerId,
						Params:     conditionParams(v.CCId, v.Params),
					}

					t.publish(e)
				}

				for _, v := range entity.EquipItemMap {
					if !t.entityCache.addOrUpdateEquipItem(entity.Id, v) {
						continue
					}

					e := &eventEntityEquipItem{
						eventBase: eventBase{
							EventId: eventIdEntityEquipItem,
							At:      p.At.Unix(),
							Id:      strconv.FormatUint(entity.Id, 10),
						},
						PocketType: v.PocketType,
						ItemId:     v.ItemId,
						Color1:     fmt.Sprintf("#%06x", v.Color1),
						Color2:     fmt.Sprintf("#%06x", v.Color2),
						Color3:     fmt.Sprintf("#%06x", v.Color3),
						Color5:     fmt.Sprintf("#%06x", v.Color5),
						Color6:     fmt.Sprintf("#%06x", v.Color6),
						Color7:     fmt.Sprintf("#%06x", v.Color7),
					}

					t.publish(e)
				}

				for _, pocketType := range t.entityCache.allEquipItemPockets(entity.Id) {
					if entity.EquipItemMap[pocketType] != nil {
						continue
					}

					t.entityCache.unequipItem(entity.Id, pocketType)

					e := &eventEntityUnequipItem{
						eventBase: eventBase{
							EventId: eventIdEntityUnequipItem,
							At:      p.At.Unix(),
							Id:      strconv.FormatUint(entity.Id, 10),
						},
						PocketType: pocketType,
					}

					t.publish(e)
				}

				continue

			case opcodeStatUpdatePrivate:
				// 私人狀態更新會發給本機玩家「與其寵物/召喚物」,
				// 只有 PC 種族的實體才能認定為本機玩家
				if p.Id == 0 || t.localPcId == p.Id {
					continue
				}

				var localInfo *packet.EntityInfo
				t.Lock()
				if e := t.entityCache[p.Id]; e != nil && e.EntityInfo != nil && e.IsUser() {
					t.localPcId = p.Id
					if !e.IsLocalPC {
						e.IsLocalPC = true
						localInfo = e.EntityInfo
					}
				}
				t.Unlock()

				if localInfo != nil {
					// 已出現過的本機玩家: 重發帶 IsLocalPC 的 appear
					t.publish(toEventEntityAppear(p.At.Unix(), localInfo))
				}

				continue

			case opcodeEntityDisappear:
				if len(p.Msg) < 1 || p.Msg[0].Type() != packet.MessageElemTypeLong {
					logger.Println("invalid packet")
					continue
				}

				id := p.Msg[0].Data().(uint64)

				t.Lock()
				t.entityCache.disappear(id, p.At)
				t.Unlock()

				e := &eventEntityDisappear{
					eventBase: eventBase{
						EventId: eventIdEntityDisappear,
						At:      p.At.Unix(),
						Id:      strconv.FormatUint(id, 10),
					},
				}
				t.publish(e)

				continue

			case OpcodeCreatureBodyUpdate:
				if len(p.Msg) < 1 || p.Msg[0].Type() != packet.MessageElemTypeBin {
					logger.Println("invalid packet")
					continue
				}

				b := p.Msg[0].Data().([]byte)

				height := math.Float32frombits(le.Uint32(b[0:]))
				weight := math.Float32frombits(le.Uint32(b[4:]))
				upper := math.Float32frombits(le.Uint32(b[8:]))
				lower := math.Float32frombits(le.Uint32(b[12:]))

				t.entityCache.updateBody(p.Id, height, weight, upper, lower)

				e := &eventEntityUpdateBody{
					eventBase: eventBase{
						EventId: eventIdEntityUpdateBody,
						At:      p.At.Unix(),
						Id:      strconv.FormatUint(p.Id, 10),
					},
					Height: height,
					Weight: weight,
					Upper:  upper,
					Lower:  lower,
				}

				t.publish(e)

				continue

			case opcodeEntitiesAppear:
				entities, err := packet.ParseEntitiesAppearPacket(p)
				if err != nil {
					logger.Println("ParseEntitiesAppearPacket failed:", err)
					continue
				}

				// 先全部入快取,再解析人偶主人 (主人可能就在同一包內)
				valid := entities[:0]
				for _, entity := range entities {
					if len(entity.Name) <= 0 || entity.Name[0] == '_' {
						// ignore npc
						continue
					}

					if entity.Id == t.localPcId {
						entity.IsLocalPC = true
					}

					t.Lock()
					// 複數路徑無法區分完整/部分實體,一律以快取補齊零值欄位
					t.entityCache.mergeFromCache(entity)
					t.entityCache.add(entity, p.At)
					t.Unlock()

					valid = append(valid, entity)
				}

				for _, entity := range valid {
					t.Lock()
					t.resolveMarionetteOwner(entity)
					t.Unlock()

					e := toEventEntityAppear(p.At.Unix(), entity)

					t.publish(e)
				}
				continue

			case opcodeEntitiesDisappear:
				if len(p.Msg) < 1 || p.Msg[0].Type() != packet.MessageElemTypeShort {
					logger.Println("invalid packet")
					continue
				}

				count := int(p.Msg[0].Data().(uint16))
				msg := p.Msg[1:]

				now := p.At.Unix()
				for i := 0; i < count; i++ {
					// ttype, id, unk1 (if ttype == 16)
					if len(msg) < 2 ||
						msg[0].Type() != packet.MessageElemTypeShort ||
						msg[1].Type() != packet.MessageElemTypeLong {

						logger.Println("invalid packet")

						for j, m := range p.Msg {
							logger.Println("* msg", j, m.Type(), m.String())
						}

						break
					}

					ttype := msg[0].Data().(uint16)
					id := msg[1].Data().(uint64)

					t.Lock()
					t.entityCache.disappear(id, p.At)
					t.Unlock()

					e := &eventEntityDisappear{
						eventBase: eventBase{
							EventId: eventIdEntityDisappear,
							At:      now,
							Id:      strconv.FormatUint(id, 10),
						},
					}
					t.publish(e)

					msg = msg[2:]

					if ttype == 16 && len(msg) >= 1 {
						msg = msg[1:]
					}
				}
				continue

			case opcodeEquipmentChanged:
				if len(p.Msg) < 1 || p.Msg[0].Type() != packet.MessageElemTypeBin {
					logger.Println("invalid packet", p.Op)
					continue
				}

				b := p.Msg[0].Data().([]byte)
				info, err := packet.EntityItemReader(b)
				if err != nil {
					logger.Println("EntityItemReader failed:", err)
					continue
				}

				if !t.entityCache.addOrUpdateEquipItem(p.Id, info) {
					continue
				}

				e := &eventEntityEquipItem{
					eventBase: eventBase{
						EventId: eventIdEntityEquipItem,
						At:      p.At.Unix(),
						Id:      strconv.FormatUint(p.Id, 10),
					},
					PocketType: info.PocketType,
					ItemId:     info.ItemId,
					Color1:     fmt.Sprintf("#%06x", info.Color1),
					Color2:     fmt.Sprintf("#%06x", info.Color2),
					Color3:     fmt.Sprintf("#%06x", info.Color3),
					Color5:     fmt.Sprintf("#%06x", info.Color5),
					Color6:     fmt.Sprintf("#%06x", info.Color6),
					Color7:     fmt.Sprintf("#%06x", info.Color7),
				}

				t.publish(e)

				continue

			case opcodeUnequipment:
				if len(p.Msg) < 1 || p.Msg[0].Type() != packet.MessageElemTypeInt {
					continue
				}

				pocketType := p.Msg[0].Data().(uint32)

				if !t.entityCache.hasEquipItem(p.Id, pocketType) {
					continue
				}

				t.entityCache.unequipItem(p.Id, pocketType)

				e := &eventEntityUnequipItem{
					eventBase: eventBase{
						EventId: eventIdEntityUnequipItem,
						At:      p.At.Unix(),
						Id:      strconv.FormatUint(p.Id, 10),
					},
					PocketType: pocketType,
				}

				t.publish(e)

				continue

			case opcodeSetFinisher:
				// set finisher
				if len(p.Msg) < 1 || p.Msg[0].Type() != packet.MessageElemTypeLong {
					logger.Println("invalid packet")
					continue
				}

				attackerId := p.Msg[0].Data().(uint64)
				attackerIdStr := ""
				if attackerId != 0 {
					attackerIdStr = strconv.FormatUint(attackerId, 10)
				}

				e := &eventFinish{
					eventBase: eventBase{
						EventId: eventIdFinish,
						At:      p.At.Unix(),
						Id:      strconv.FormatUint(p.Id, 10),
					},
					AttackerId: attackerIdStr,
				}
				t.publish(e)

				continue

			case opcodeCombatAction:
				pack, err := packet.ParseCombatActionPackPacket(p)
				if err != nil {
					logger.Println("ParseCombatActionPackPacket failed:", err)
					continue
				}

				attackerId := uint64(0)
				attackSkillId := uint16(0)

				// find attacker
				for i, v := range pack.SubPackets {
					_ = i

					if debug {
						logger.Println("sub packet", i, v.Hit != nil, v.Attacker != nil)
						logger.Printf("base %+v", v)
						if v.Hit != nil {
							logger.Printf("hit %+v", v.Hit)
						}

						if v.Attacker != nil {
							logger.Printf("attacker %+v", v.Attacker)
						}
					}

					// 한패킷에 공격자가 2명 이상 일 수 있을까?
					if v.Hit == nil {
						// 공격자
						attackerId = v.EntityId
						attackSkillId = v.SkillId
						break
					}
				}

				for _, v := range pack.SubPackets {
					if v.Hit == nil {
						continue
					}

					// 방어자
					targetId := v.EntityId
					damage := v.Hit.Damage
					isCritical := v.Hit.Options&0x1 != 0

					e := &eventDamage{
						eventBase: eventBase{
							EventId: eventIdDamage,
							At:      p.At.Unix(),
							Id:      strconv.FormatUint(attackerId, 10),
						},
						TargetId:   strconv.FormatUint(targetId, 10),
						SkillId:    attackSkillId,
						Damage:     damage,
						IsCritical: isCritical,
					}
					t.publish(e)
				}

				continue

			case opcodeEffectDelayed:
				// effect delayed, 연공 블래스트 대미지가 이걸로 날라옴
				targetId := p.Id

				if len(p.Msg) < 2 ||
					p.Msg[0].Type() != packet.MessageElemTypeInt ||
					p.Msg[1].Type() != packet.MessageElemTypeInt {

					for i, msg := range p.Msg {
						logger.Println("* msg", i, msg.Type(), msg.String())
					}

					logger.Println("invalid packet")
					continue
				}

				delay := p.Msg[0].Data().(uint32)
				ttype := p.Msg[1].Data().(uint32)
				if !delayedDamageTypeSet[ttype] {
					_ = delay

					// 연공 블래스트가 아님。
					// 但若封包長得就是延遲傷害的樣子 (Int,Int,Int,Byte,Int,Long,Short),
					// 代表改版又把類型 id 位移了 —— 把號碼印出來以便補進 delayedDamageTypeSet
					if isDelayedDamageShape(p.Msg) {
						warnUnknownDelayedType(ttype)
					}

					continue
				}

				_ = delay

				if len(p.Msg) < 7 {
					logger.Println("invalid packet")
					logger.Printf("packet op %x id %x", p.Op, p.Id)
					for i, msg := range p.Msg {
						logger.Println("* msg", i, msg.Type(), msg.String())
					}
					continue
				}
				if p.Msg[2].Type() != packet.MessageElemTypeInt {
					logger.Println("invalid packet")
					logger.Printf("packet op %x id %x", p.Op, p.Id)
					for i, msg := range p.Msg {
						logger.Println("* msg", i, msg.Type(), msg.String())
					}
					continue
				}
				if p.Msg[5].Type() != packet.MessageElemTypeLong {
					logger.Println("invalid packet")
					logger.Printf("packet op %x id %x", p.Op, p.Id)
					for i, msg := range p.Msg {
						logger.Println("* msg", i, msg.Type(), msg.String())
					}
					continue
				}
				if p.Msg[6].Type() != packet.MessageElemTypeShort {
					logger.Println("invalid packet")
					logger.Printf("packet op %x id %x", p.Op, p.Id)
					for i, msg := range p.Msg {
						logger.Println("* msg", i, msg.Type(), msg.String())
					}
					continue
				}

				damage := p.Msg[2].Data().(uint32)
				attackerId := p.Msg[5].Data().(uint64)
				attackSkillId := p.Msg[6].Data().(uint16)

				e := &eventDamage{
					eventBase: eventBase{
						EventId: eventIdDamage,
						At:      p.At.Unix(),
						Id:      strconv.FormatUint(attackerId, 10),
					},
					TargetId:  strconv.FormatUint(targetId, 10),
					SkillId:   attackSkillId,
					Damage:    float32(damage),
					IsDelayed: true,
				}
				t.publish(e)

				continue

			case opcodeConditionUpdate:
				// condition update
				cond, err := packet.ParseCharacterConditionPacket(p)
				if err != nil {
					logger.Println("ParseCharacterConditionPacket failed:", err)
					continue
				}

				t.Lock()
				t.entityCache.addCondition(cond)
				t.Unlock()

				if !cond.IsEnable {
					e := &eventCharacterConditionDisable{
						eventBase: eventBase{
							EventId: eventIdCharacterConditionDisable,
							At:      p.At.Unix(),
							Id:      strconv.FormatUint(cond.Id, 10),
						},
						CCId: cond.CCId,
					}
					t.publish(e)
					continue
				}

				attackerId := ""
				if cond.AttackerId != 0 {
					attackerId = strconv.FormatUint(cond.AttackerId, 10)
				}

				e := &eventCharacterConditionEnable{
					eventBase: eventBase{
						EventId: eventIdCharacterConditionEnable,
						At:      p.At.Unix(),
						Id:      strconv.FormatUint(cond.Id, 10),
					},
					CCId:       cond.CCId,
					DisableAt:  cond.DisableAt,
					AttackerId: attackerId,
					Params:     conditionParams(cond.CCId, cond.Params),
				}
				t.publish(e)

				continue
			}
		}
	}
}

func (t *eventPublisher) publish(e iEvent) {
	// blocking이 되면 안된다

	t.Lock()
	defer t.Unlock()

	for k, c := range t.clientMap {
		select {
		case <-c.ctx.Done():
			delete(t.clientMap, k)
			continue

		default:
			_ = 1
		}

		select {
		case c.ch <- e:
			// write ok
			_ = 1

		default:
			// queue full
			delete(t.clientMap, k)
			logger.Println("queue full... force close socket", k)
			continue
		}
	}
}

func (t *eventPublisher) addClient(ctx context.Context, ch chan<- iEvent) uint32 {
	t.Lock()
	t.currentClientId++
	clientId := t.currentClientId
	t.Unlock()

	// bulk write 필요할듯
	events := []iEvent(nil)

	t.Lock()
	for _, entity := range t.entityCache {
		e := toEventEntityAppear(entity.appearAt, entity.EntityInfo)

		events = append(events, e)

		for _, cond := range entity.characterConditionMap {
			attackerId := ""
			if cond.AttackerId != 0 {
				attackerId = strconv.FormatUint(cond.AttackerId, 10)
			}

			e := &eventCharacterConditionEnable{
				eventBase: eventBase{
					EventId: eventIdCharacterConditionEnable,
					At:      entity.appearAt,
					Id:      strconv.FormatUint(entity.Id, 10),
				},
				CCId:       cond.CCId,
				DisableAt:  cond.DisableAt,
				AttackerId: attackerId,
				Params:     conditionParams(cond.CCId, cond.Params),
			}

			events = append(events, e)
		}

		for _, item := range entity.equipItemMap {
			e := &eventEntityEquipItem{
				eventBase: eventBase{
					EventId: eventIdEntityEquipItem,
					At:      entity.appearAt,
					Id:      strconv.FormatUint(entity.Id, 10),
				},
				PocketType: item.PocketType,
				ItemId:     item.ItemId,
				Color1:     fmt.Sprintf("#%06x", item.Color1),
				Color2:     fmt.Sprintf("#%06x", item.Color2),
				Color3:     fmt.Sprintf("#%06x", item.Color3),
				Color5:     fmt.Sprintf("#%06x", item.Color5),
				Color6:     fmt.Sprintf("#%06x", item.Color6),
				Color7:     fmt.Sprintf("#%06x", item.Color7),
			}

			events = append(events, e)
		}
	}
	t.Unlock()

	logger.Println("send initial data", clientId, ", ", len(events), "events")

	for _, e := range events {
		ch <- e
	}

	t.Lock()
	t.clientMap[clientId] = &eventClient{
		ctx: ctx,
		ch:  ch,
	}
	t.Unlock()

	return clientId
}

func toEventEntityAppear(now int64, p *packet.EntityInfo) *eventEntityAppear {
	ownerId := ""

	if p.OwnerId != 0 {
		ownerId = strconv.FormatUint(p.OwnerId, 10)
	}

	v := &eventEntityAppear{
		eventBase: eventBase{
			EventId: eventIdEntityAppear,
			At:      now,
			Id:      strconv.FormatUint(p.Id, 10),
		},
		Name:      p.Name,
		RaceId:    p.RaceId,
		Height:    p.Height,
		Weight:    p.Weight,
		Upper:     p.Upper,
		Lower:     p.Lower,
		GuildName: p.GuildName,
		OwnerId:   ownerId,
		IsLocalPC: p.IsLocalPC,
	}

	return v
}
