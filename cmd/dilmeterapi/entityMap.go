package main

import (
	"time"

	"gitlab.com/prilus/mabidilmeter/packet"
)

type entityCache map[uint64]*entityInfoExtend

type entityInfoExtend struct {
	*packet.EntityInfo
	DisappearAt int64
}

func (t entityCache) add(e *packet.EntityInfo) {
	if e, ok := t[e.Id]; ok {
		e.DisappearAt = 0
		return
	}

	t[e.Id] = &entityInfoExtend{
		EntityInfo:  e,
		DisappearAt: 0,
	}
	t.cleanup()
}

func (t entityCache) disappear(id uint64) {
	// at을 외부에서 받아야할 수 도 있음 생각보다 느림
	at := time.Now().Unix()

	if v, ok := t[id]; ok {
		v.DisappearAt = at
	}
}

func (t entityCache) cleanup() {
	now := time.Now().Unix()
	mobRemoveSec, userRemoveSec := int64(1*60), int64(5*60)

	for k, v := range t {
		if v.DisappearAt == 0 {
			continue
		}

		if v.IsUser() {
			if now-v.DisappearAt > userRemoveSec {
				delete(t, k)
			}
			continue
		}

		if now-v.DisappearAt > mobRemoveSec {
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
