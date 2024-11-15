package main

import (
	"sync"
	"time"

	"gitlab.com/prilus/mabidilmeter/packet"
)

type entityCache map[uint64]*entityInfoExtend

type entityInfoExtend struct {
	*packet.EntityInfo
	sync.Mutex
	appearAt              int64
	disappearAt           int64
	characterConditionMap map[uint32]*entityCharacterCondition
}

type entityCharacterCondition struct {
	CCId       uint32
	DisableAt  int64
	AttackerId uint64
}

func (t entityCache) add(p *packet.EntityInfo) {
	at := time.Now().Unix()

	if e, ok := t[p.Id]; ok {
		e.appearAt = at
		e.disappearAt = 0
		return
	}

	t[p.Id] = &entityInfoExtend{
		EntityInfo:            p,
		appearAt:              at,
		disappearAt:           0,
		characterConditionMap: make(map[uint32]*entityCharacterCondition),
	}
	t.cleanup()
}

func (t entityCache) disappear(id uint64) {
	// at을 외부에서 받아야할 수 도 있음 생각보다 느림
	at := time.Now().Unix()

	e := t[id]
	if e == nil {
		return
	}

	e.disappearAt = at
}

func (t entityCache) addCondition(p *packet.CharacterConditionPacket) {
	e := t[p.Id]
	if e == nil {
		return
	}

	if !p.IsEnable {
		e.Lock()
		delete(e.characterConditionMap, p.CCId)
		e.Unlock()
		return
	}

	newCond := &entityCharacterCondition{
		CCId:       p.CCId,
		DisableAt:  p.DisableAt,
		AttackerId: p.AttackerId,
	}

	e.Lock()
	e.characterConditionMap[p.CCId] = newCond
	e.Unlock()

	go func() {
		time.Sleep(time.Until(time.Unix(p.DisableAt, 0)))
		e.Lock()
		if cond := e.characterConditionMap[p.CCId]; cond != nil && cond.DisableAt == p.DisableAt {
			delete(e.characterConditionMap, p.CCId)
		}
		e.Unlock()
	}()
}

func (t entityCache) cleanup() {
	now := time.Now().Unix()
	mobRemoveSec, userRemoveSec := int64(1*60), int64(5*60)
	noDisappearMobRemoveSec, noDisappearUserRemoveSec := int64(12*60*60), int64(3*60*60)

	for k, v := range t {
		if v.disappearAt == 0 {
			if v.IsUser() {
				if now-v.appearAt > noDisappearUserRemoveSec {
					delete(t, k)
				}
			}

			if now-v.appearAt > noDisappearMobRemoveSec {
				delete(t, k)
			}

			continue
		}

		if v.IsUser() {
			if now-v.disappearAt > userRemoveSec {
				delete(t, k)
			}
			continue
		}

		if now-v.disappearAt > mobRemoveSec {
			delete(t, k)
		}
	}
}

func (t *entityInfoExtend) IsUser() bool {
	switch t.RaceId {
	case 8001, 8002, 9001, 9002, 10001, 10002:
		return true
	}

	return false
}
