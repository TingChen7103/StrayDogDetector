package packet

import (
	"bytes"
	"fmt"

	"gitlab.com/prilus/mabidilmeter/util"
)

type EntityInfo struct {
	Id                    uint64
	Name                  string
	RaceId                uint32
	SkinColor             uint8
	EyeType               uint16
	LeftEyeColor          uint8
	RightEyeColor         uint8
	MouthType             uint16
	Height                float32
	Weight                float32
	Upper                 float32
	Lower                 float32
	TitleId               uint32
	SubTitleId            uint32
	StyleTitleId          uint32
	StyleSubTitleId       uint32
	EquipItemMap          map[uint32]*EntityItem
	CharacterConditionMap map[uint32]*EntityCharacterCondition
	GuildName             string
	OwnerId               uint64 // 펫, 마리오네트 등
	IsLocalPC             bool

	// 人偶類版型的主人 id 位於標準解析終點之後的延伸區塊,
	// 位置隨裝備數浮動;此處收集殘餘元素中的非零 Long 供上層以實體快取驗證
	OwnerCandidates []uint64
}

type EntityItem struct {
	// public data
	PocketType uint32
	ItemId     uint32
	Color1     uint32
	Color2     uint32
	Color3     uint32
	Color4     uint32
	Color5     uint32
	Color6     uint32
	Color7     uint32
	Amount     uint16
}

type EntityCharacterCondition struct {
	CCId       uint32
	DisableAt  int64
	AttackerId uint64

	// 效果參數字串,格式 `鍵:型別:值;` (型別 f=float、b=bool、8=時間戳)。
	// 例如戰場的序曲: `MCMBAMIN:f:66.363777;MCMBAMAX:f:66.363777;MCMBAC:f:34.928299;...`
	// 只有需要顯示數值的條件才會被填入 (見 eventPublisher 的 conditionParamCCIds)
	Params string
}

func ParseEntityAppearPacket(msg Message) (retV *EntityInfo, retErr error) {
	// 異常封包不可讓整個行程崩潰
	defer func() {
		if r := recover(); r != nil {
			retV, retErr = nil, fmt.Errorf("ParseEntityAppearPacket panic: %v", r)
		}
	}()

	origMsg := msg

	curPos := func() int {
		return len(origMsg) - len(msg)
	}

	if len(msg) < 2 || msg[1].Type() != MessageElemTypeByte {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return nil, err
	}

	appearType := msg[1].Data().(uint8)
	if appearType != 5 && appearType != 4 && appearType != 3 {
		// public (5), self (4), pet/marionette (3) character data만 읽음
		return nil, nil
	}

	v := &EntityInfo{
		EquipItemMap:          make(map[uint32]*EntityItem),
		CharacterConditionMap: make(map[uint32]*EntityCharacterCondition),
		IsLocalPC:             appearType == 4,
	}

	if len(msg) < 40 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		return v, err
	}

	if msg[0].Type() != MessageElemTypeLong {
		err := fmt.Errorf("id has unexpected type %v", msg[0].Type())
		logger.Println(err)
		return v, err
	}

	v.Id = msg[0].Data().(uint64)

	if msg[2].Type() != MessageElemTypeString {
		err := fmt.Errorf("name has unexpected type %v", msg[2].Type())
		logger.Println(err)
		return v, err
	}

	v.Name = msg[2].Data().(string)

	if msg[5].Type() != MessageElemTypeInt {
		err := fmt.Errorf("raceId has unexpected type %v", msg[5].Type())
		logger.Println(err)
		return v, err
	}

	v.RaceId = msg[5].Data().(uint32)

	if msg[6].Type() != MessageElemTypeByte {
		err := fmt.Errorf("skinColor has unexpected type %v", msg[6].Type())
		logger.Println(err)
		return v, err
	}

	v.SkinColor = msg[6].Data().(uint8)

	if msg[7].Type() != MessageElemTypeShort {
		err := fmt.Errorf("eyeType has unexpected type %v", msg[7].Type())
		logger.Println(err)
		return v, err
	}

	v.EyeType = msg[7].Data().(uint16)

	if msg[8].Type() != MessageElemTypeByte {
		err := fmt.Errorf("eyeColor has unexpected type %v", msg[8].Type())
		logger.Println(err)
		return v, err
	}

	eyeColor := msg[8].Data().(uint8)

	if msg[9].Type() != MessageElemTypeShort {
		err := fmt.Errorf("mouthType has unexpected type %v", msg[9].Type())
		logger.Println(err)
		return v, err
	}

	v.MouthType = msg[9].Data().(uint16)

	if msg[13].Type() != MessageElemTypeFloat {
		err := fmt.Errorf("height has unexpected type %v", msg[13].Type())
		logger.Println(err)
		return v, err
	}

	v.Height = msg[13].Data().(float32)

	if msg[14].Type() != MessageElemTypeFloat {
		err := fmt.Errorf("weight has unexpected type %v", msg[14].Type())
		logger.Println(err)
		return v, err
	}

	v.Weight = msg[14].Data().(float32)

	if msg[15].Type() != MessageElemTypeFloat {
		err := fmt.Errorf("upper has unexpected type %v", msg[15].Type())
		logger.Println(err)
		return v, err
	}

	v.Upper = msg[15].Data().(float32)

	if msg[16].Type() != MessageElemTypeFloat {
		err := fmt.Errorf("lower has unexpected type %v", msg[16].Type())
		logger.Println(err)
		return v, err
	}

	v.Lower = msg[16].Data().(float32)

	if msg[28].Type() != MessageElemTypeByte {
		err := fmt.Errorf("leftEyeColor has unexpected type %v", msg[28].Type())
		logger.Println(err)
		return v, err
	}

	v.LeftEyeColor = msg[28].Data().(uint8)

	if v.LeftEyeColor == 0 {
		v.LeftEyeColor = eyeColor
	}

	if msg[29].Type() != MessageElemTypeByte {
		err := fmt.Errorf("rightEyeColor has unexpected type %v", msg[29].Type())
		logger.Println(err)
		return v, err
	}

	v.RightEyeColor = msg[29].Data().(uint8)

	if v.RightEyeColor == 0 {
		v.RightEyeColor = eyeColor
	}

	if msg[39].Type() != MessageElemTypeInt {
		err := fmt.Errorf("regenCount has unexpected type %v", msg[39].Type())
		logger.Println(err)
		return v, err
	}

	regenCount := msg[39].Data().(uint32)

	msg = msg[40:]

	if len(msg) < 7*int(regenCount) {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	msg = msg[7*regenCount:]

	if len(msg) < 1 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	if msg[0].Type() != MessageElemTypeInt {
		err := fmt.Errorf("regen2Count has unexpected type %v", msg[0].Type())
		logger.Println(err)
		return v, err
	}

	regen2Count := msg[0].Data().(uint32)
	msg = msg[1:]

	if len(msg) < 7*int(regen2Count) {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	msg = msg[7*regen2Count:]

	if len(msg) < 10 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	if msg[0].Type() != MessageElemTypeInt {
		err := fmt.Errorf("titleId has unexpected type %v", msg[0].Type())
		logger.Println(err)
		return v, err
	}

	v.TitleId = msg[0].Data().(uint32)

	if msg[2].Type() != MessageElemTypeInt {
		err := fmt.Errorf("subTitleId has unexpected type %v", msg[2].Type())
		logger.Println(err)
		return v, err
	}

	v.SubTitleId = msg[2].Data().(uint32)

	if msg[3].Type() != MessageElemTypeInt {
		err := fmt.Errorf("styleTitleId has unexpected type %v", msg[3].Type())
		logger.Println(err)
		return v, err
	}

	v.StyleTitleId = msg[3].Data().(uint32)

	if msg[4].Type() != MessageElemTypeInt {
		err := fmt.Errorf("styleSubTitleId has unexpected type %v", msg[4].Type())
		logger.Println(err)
		return v, err
	}

	v.StyleSubTitleId = msg[4].Data().(uint32)

	if msg[9].Type() != MessageElemTypeInt {
		err := fmt.Errorf("unk1Count has unexpected type %v", msg[9].Type())
		logger.Println(err)
		return v, err
	}

	unk1Count := msg[9].Data().(uint32)
	msg = msg[10:]

	if len(msg) < 2*int(unk1Count) {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	msg = msg[2*unk1Count:]

	if len(msg) < 1 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	if msg[0].Type() != MessageElemTypeInt {
		err := fmt.Errorf("equipItemCount has unexpected type %v", msg[0].Type())
		logger.Println(err)
		return v, err
	}

	equipItemCount := int(msg[0].Data().(uint32))
	if len(msg) < 2*equipItemCount {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	msg = msg[1:]

	for i := 0; i < equipItemCount; i, msg = i+1, msg[2:] {
		// 每件裝備佔 2 個元素,공회 로브 會多吃 1 個,總長無法預先算準,逐件檢查
		if len(msg) < 2 {
			err := fmt.Errorf("entity appear data is too short %v", curPos())
			logger.Println(err)
			return v, err
		}

		if msg[1].Type() != MessageElemTypeBin {
			err := fmt.Errorf("equipItemData has unexpected type %v", msg[1].Type())
			logger.Println(err)
			return v, err
		}

		b := msg[1].Data().([]byte)
		d, err := EntityItemReader(b)
		if err != nil {
			logger.Println("EntityItemReader failed:", err, i)
			return v, err
		}

		v.EquipItemMap[d.PocketType] = d

		if len(msg) > 2 && msg[2].Type() == MessageElemTypeString {
			// 길드 로브
			msg = msg[1:]
		}
	}

	// 스킬 관련
	if len(msg) < 4 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	if msg[3].Type() != MessageElemTypeInt {
		err := fmt.Errorf("skillCount has unexpected type %v", msg[3].Type())
		logger.Println(err)
		return v, err
	}

	skillCount := int(msg[3].Data().(uint32))
	msg = msg[4:]

	if len(msg) < skillCount {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	msg = msg[skillCount:]

	// unknown field
	if len(msg) < 2 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	msg = msg[2:]

	// 파티 관련
	if len(msg) < 2 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	msg = msg[2:]

	// pvp 관련
	if len(msg) < 16 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	msg = msg[16:]

	// 컨디션 관련
	if len(msg) < 3 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	if msg[2].Type() != MessageElemTypeInt {
		err := fmt.Errorf("conditionCount has unexpected type %v", msg[2].Type())
		logger.Println(err)
		return v, err
	}

	conditionCount := int(msg[2].Data().(uint32))
	msg = msg[3:]

	if len(msg) < (conditionCount * 6) {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	for i := 0; i < conditionCount; i, msg = i+1, msg[6:] {
		/*
			uint32 ccId
			uint64 disableAt
			string metadata 나중에 필요할 수 도 있음
			uint64 attackerId
			string unknown1
			string 해제시 메세지?
		*/

		if msg[0].Type() != MessageElemTypeInt {
			err := fmt.Errorf("ccId has unexpected type %v", msg[0].Type())
			logger.Println(err)
			return v, err
		}

		ccId := msg[0].Data().(uint32)

		if msg[1].Type() != MessageElemTypeLong {
			err := fmt.Errorf("disableAt has unexpected type %v", msg[1].Type())
			logger.Println(err)
			return v, err
		}

		disableAtRaw := msg[1].Data().(uint64)
		disableAt := util.ParseMabiTime(disableAtRaw).Unix()

		if msg[3].Type() != MessageElemTypeLong {
			err := fmt.Errorf("attackerId has unexpected type %v", msg[3].Type())
			logger.Println(err)
			return v, err
		}

		attackerId := msg[3].Data().(uint64)

		// msg[2] 是效果參數字串 (音樂 buff 的實際加成數值就在這裡)
		params := ""
		if msg[2].Type() == MessageElemTypeString {
			params = msg[2].Data().(string)
		}

		v.CharacterConditionMap[ccId] = &EntityCharacterCondition{
			CCId:       ccId,
			DisableAt:  disableAt,
			AttackerId: attackerId,
			Params:     params,
		}
	}

	// unknown field
	if len(msg) < 1 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	msg = msg[1:]

	// 길드 관련
	if len(msg) < 19 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	if msg[1].Type() != MessageElemTypeString {
		err := fmt.Errorf("guildName has unexpected type %v", msg[1].Type())
		logger.Println(err)
		return v, err
	}

	v.GuildName = msg[1].Data().(string)
	msg = msg[19:]

	// 펫 관련
	if len(msg) < 2 {
		err := fmt.Errorf("entity appear data is too short %v", curPos())
		logger.Println(err)
		return v, err
	}

	if msg[1].Type() != MessageElemTypeLong {
		err := fmt.Errorf("ownerId has unexpected type %v", msg[1].Type())
		logger.Println(err)
		return v, err
	}

	v.OwnerId = msg[1].Data().(uint64)
	msg = msg[2:]

	if v.OwnerId == 0 {
		for _, elem := range msg {
			if elem.Type() != MessageElemTypeLong {
				continue
			}

			id := elem.Data().(uint64)
			if id != 0 && id != v.Id {
				v.OwnerCandidates = append(v.OwnerCandidates, id)
			}
		}
	}

	return v, nil
}

func ParseEntitiesAppearPacket(p *GamePacket) (entities []*EntityInfo, retErr error) {
	// 異常封包不可讓整個行程崩潰;已解析出的實體照樣回傳
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("ParseEntitiesAppearPacket panic: %v", r)
		}
	}()

	msg := p.Msg
	if len(msg) < 1 || msg[0].Type() != MessageElemTypeShort {
		return nil, fmt.Errorf("invalid packet")
	}

	count := int(msg[0].Data().(uint16))
	msg = msg[1:]

	for i := 0; i < count; i++ {
		if len(msg) < 3 {
			break
		}

		if msg[0].Type() != MessageElemTypeShort ||
			msg[1].Type() != MessageElemTypeInt ||
			msg[2].Type() != MessageElemTypeBin {

			// 條目結構不符,無法安全前進,放棄剩餘條目
			logger.Println("invalid packet", i)
			break
		}

		t, b := msg[0].Data().(uint16), msg[2].Data().([]byte)
		msg = msg[3:]

		if t != 16 {
			// 캐릭터가 아님
			// logger.Println("invalid packet", i, t)
			continue
		}

		_, _, subMsg, err := GamePacketBodyReader(bytes.NewReader(b))
		if err != nil {
			logger.Println("GamePacketBodyReader failed:", err)
			continue
		}

		v, err := ParseEntityAppearPacket(subMsg)
		if err != nil {
			logger.Println("ParseEntityAppearPacket failed:", err)
			if v == nil || v.Name == "" || v.RaceId == 0 {
				continue
			}
		}

		if v != nil {
			entities = append(entities, v)
		}

	}

	return entities, nil
}

func EntityItemReader(b []byte) (*EntityItem, error) {
	r := new(EntityItem)
	if len(b) < 38 {
		err := fmt.Errorf("item public info data is too short %v", len(b))
		return nil, err
	}

	r.PocketType = le.Uint32(b[0:]) // uint8일듯?
	r.ItemId = le.Uint32(b[4:])
	r.Color1 = le.Uint32(b[8:])
	r.Color2 = le.Uint32(b[12:])
	r.Color3 = le.Uint32(b[16:])
	r.Color4 = le.Uint32(b[20:])
	r.Color5 = le.Uint32(b[24:])
	r.Color6 = le.Uint32(b[28:])
	r.Color7 = le.Uint32(b[32:])
	r.Amount = le.Uint16(b[36:])
	if r.Amount == 0 {
		r.Amount = 1
	}

	return r, nil
}
