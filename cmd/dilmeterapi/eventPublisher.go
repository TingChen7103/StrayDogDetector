package main

import (
	"context"
	"strconv"
	"sync"
	"time"

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
}

type eventClient struct {
	ctx context.Context
	ch  chan<- iEvent
}

const (
	opcodeEntityAppear      = 0x520c
	opcodeEntityDisappear   = 0x520d
	opcodeEntitiesAppear    = 0x5334
	opcodeEntitiesDisappear = 0x5335
	opcodeCombatAction      = 0x7926
	opcodeEffectDelayed     = 0x9092
)

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
				if err != nil {
					logger.Println("ParseEntityAppearPacket failed:", err)
					continue
				}

				if entity.Name[0] == '_' {
					// ignore npc
					continue
				}

				t.Lock()
				t.entityCache.add(entity)
				t.Unlock()

				e := &eventEntityAppear{
					eventBase: eventBase{
						EventId: eventIdEntityAppear,
						At:      time.Now().Unix(),
					},
					Id:     strconv.FormatUint(entity.Id, 10),
					Name:   entity.Name,
					RaceId: entity.RaceId,
				}
				t.publish(e)

				continue

			case 0x520d:
				if len(p.Msg) < 1 || p.Msg[0].Type() != packet.MessageElemTypeLong {
					logger.Println("invalid packet")
					continue
				}

				id := p.Msg[0].Data().(uint64)

				t.Lock()
				t.entityCache.disappear(id)
				t.Unlock()

				e := &eventEntityDisappear{
					eventBase: eventBase{
						EventId: eventIdEntityDisappear,
						At:      time.Now().Unix(),
					},
					Id: strconv.FormatUint(id, 10),
				}
				t.publish(e)

				continue

			case opcodeEntitiesAppear:
				entities, err := packet.ParseEntitiesAppearPacket(p)
				if err != nil {
					logger.Println("ParseEntitiesAppearPacket failed:", err)
					continue
				}

				now := time.Now().Unix()
				for _, entity := range entities {
					if entity.Name[0] == '_' {
						// ignore npc
						continue
					}

					t.Lock()
					t.entityCache.add(entity)
					t.Unlock()

					e := &eventEntityAppear{
						eventBase: eventBase{
							EventId: eventIdEntityAppear,
							At:      now,
						},
						Id:     strconv.FormatUint(entity.Id, 10),
						Name:   entity.Name,
						RaceId: entity.RaceId,
					}
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

				now := time.Now().Unix()
				for i := 0; i < count; i++ {
					// ttype, id, unk1 (if ttype == 16)
					if len(msg) < 2 || msg[1].Type() != packet.MessageElemTypeLong {
						logger.Println("invalid packet")

						for j, m := range p.Msg {
							logger.Println("* msg", j, m.Type(), m.String())
						}

						break
					}

					ttype := msg[0].Data().(uint8)
					id := msg[1].Data().(uint64)

					t.Lock()
					t.entityCache.disappear(id)
					t.Unlock()

					e := &eventEntityDisappear{
						eventBase: eventBase{
							EventId: eventIdEntityDisappear,
							At:      now,
						},
						Id: strconv.FormatUint(id, 10),
					}
					t.publish(e)

					msg = msg[2:]

					if ttype == 16 && len(msg) < 1 {
						msg = msg[1:]
					}
				}
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
							At:      time.Now().Unix(),
						},
						Id:         strconv.FormatUint(attackerId, 10),
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

				if len(p.Msg) < 1 && p.Msg[0].Type() != packet.MessageElemTypeInt {
					logger.Println("invalid packet")
					continue
				}

				ttype := p.Msg[0].Data().(uint32)
				if ttype != 0 {
					// 연공 블래스트가 아님
					continue
				}

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
						At:      time.Now().Unix(),
					},
					Id:       strconv.FormatUint(attackerId, 10),
					TargetId: strconv.FormatUint(targetId, 10),
					SkillId:  attackSkillId,
					Damage:   float32(damage),
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
			continue
		}
	}
}

func (t *eventPublisher) addClient(ctx context.Context, ch chan<- iEvent) uint32 {
	t.Lock()
	defer t.Unlock()

	t.currentClientId++
	clientId := t.currentClientId
	t.clientMap[clientId] = &eventClient{
		ctx: ctx,
		ch:  ch,
	}

	now := time.Now().Unix()
	for _, entity := range t.entityCache {
		e := &eventEntityAppear{
			eventBase: eventBase{
				EventId: eventIdEntityAppear,
				At:      now,
			},
			Id:     strconv.FormatUint(entity.Id, 10),
			Name:   entity.Name,
			RaceId: entity.RaceId,
		}
		ch <- e
	}

	return clientId
}
