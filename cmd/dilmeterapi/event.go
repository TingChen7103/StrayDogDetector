package main

type eventId uint16

const (
	eventIdEntityAppear eventId = 1 + iota
	eventIdEntityDisappear
	eventIdDamage
	eventIdCharacterConditionEnable
	eventIdCharacterConditionDisable
	eventIdFinish
	eventIdEntityEquipItem
	eventIdEntityUnequipItem
	eventIdEntityUpdateBody
)

type iEvent interface {
	GetEventId() eventId
}

type eventBase struct {
	EventId eventId
	At      int64
	Id      string
}

func (t *eventBase) GetEventId() eventId {
	return t.EventId
}

type eventEntityAppear struct {
	eventBase
	Name      string
	RaceId    uint32
	Height    float32
	Weight    float32
	Upper     float32
	Lower     float32
	GuildName string
	OwnerId   string
	IsLocalPC bool
}

type eventEntityDisappear struct {
	eventBase
}

type eventDamage struct {
	eventBase
	TargetId   string
	SkillId    uint16
	Damage     float32
	IsCritical bool
	IsDelayed  bool
}

type eventCharacterConditionEnable struct {
	eventBase
	CCId       uint32
	DisableAt  int64
	AttackerId string
	// 效果參數字串,只有 conditionParamCCIds 內的條件才會帶
	// (94% 的條件都有參數,全部輸出會讓 ndjson 多出 ~15% 體積)
	Params string `json:",omitempty"`
}

type eventCharacterConditionDisable struct {
	eventBase
	CCId uint32
}

type eventFinish struct {
	eventBase
	AttackerId string
}

type eventEntityEquipItem struct {
	eventBase
	PocketType uint32
	ItemId     uint32
	Color1     string
	Color2     string
	Color3     string
	Color5     string
	Color6     string
	Color7     string
}

type eventEntityUnequipItem struct {
	eventBase
	PocketType uint32
}

type eventEntityUpdateBody struct {
	eventBase
	Height float32
	Weight float32
	Upper  float32
	Lower  float32
}
