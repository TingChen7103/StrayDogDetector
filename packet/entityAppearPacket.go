package packet

import (
	"bytes"
	"fmt"
)

type EntityInfo struct {
	Id     uint64
	Name   string
	RaceId uint32
}

func ParseEntityAppearPacket(msg Message) (*EntityInfo, error) {
	if len(msg) < 6 || msg[1].Type() != MessageElemTypeByte {
		logger.Println("entity appear data is too short")
		return nil, nil
	}

	if msg[1].Data().(uint8) != 5 {
		// public 데이터만 읽음
		return nil, nil
	}

	if msg[0].Type() != MessageElemTypeLong {
		return nil, fmt.Errorf("ParseEntityAppearPacket: id has unexpected type %v", msg[0].Type())
	}
	if msg[2].Type() != MessageElemTypeString {
		return nil, fmt.Errorf("ParseEntityAppearPacket: name has unexpected type %v", msg[2].Type())
	}
	if msg[5].Type() != MessageElemTypeInt {
		return nil, fmt.Errorf("ParseEntityAppearPacket: raceId has unexpected type %v", msg[5].Type())
	}

	id := msg[0].Data().(uint64)
	name := msg[2].Data().(string)
	raceId := msg[5].Data().(uint32)

	v := &EntityInfo{
		Id:     id,
		Name:   name,
		RaceId: raceId,
	}

	return v, nil
}

func ParseEntitiesAppearPacket(p *GamePacket) ([]*EntityInfo, error) {
	entities := []*EntityInfo(nil)
	msg := p.Msg
	if len(msg) < 1 || msg[0].Type() != MessageElemTypeShort {
		return nil, fmt.Errorf("invalid packet")
	}

	count := int(msg[0].Data().(uint16))
	msg = msg[1:]

	for i := 1; i < count; i++ {
		if len(msg) < 3 {
			break
		}

		if msg[0].Type() != MessageElemTypeShort ||
			msg[1].Type() != MessageElemTypeInt ||
			msg[2].Type() != MessageElemTypeBin {

			logger.Println("invalid packet", i)
			continue
		}

		t, b := msg[0].Data().(uint16), msg[2].Data().([]byte)
		if t != 16 {
			// 캐릭터가 아님
			// logger.Println("invalid packet", i, t)
			continue
		}

		msg = msg[3:]

		_, _, subMsg, err := GamePacketBodyReader(bytes.NewReader(b))
		if err != nil {
			logger.Println("GamePacketBodyReader failed:", err)
			continue
		}

		v, err := ParseEntityAppearPacket(subMsg)
		if err != nil {
			logger.Println("ParseEntityAppearPacket failed:", err)
			continue
		}

		if v != nil {
			entities = append(entities, v)
		}

	}

	return entities, nil
}
