package main

type eventId uint16

const (
	eventIdEntityAppear eventId = 1 + iota
	eventIdEntityDisappear
	eventIdDamage
)

type iEvent interface {
	GetEventId() eventId
}

type eventBase struct {
	EventId eventId
	At      int64
}

func (t *eventBase) GetEventId() eventId {
	return t.EventId
}

type eventEntityAppear struct {
	eventBase
	Id     string
	Name   string
	RaceId uint32
}

type eventEntityDisappear struct {
	eventBase
	Id string
}

type eventDamage struct {
	eventBase
	Id         string
	TargetId   string
	SkillId    uint16
	Damage     float32
	IsCritical bool
}
