export type eventId = number;

export const eventIdEntityAppear = 1;
export const eventIdEntityDisappear = 2;
export const eventIdDamage = 3;
export const eventIdCharacterConditionEnable = 4;
export const eventIdCharacterConditionDisable = 5;
export const eventIdFinish = 6;

export type eventBase = {
    EventId: eventId;
    At: number;
    Id: string;
}

export type eventEntityAppear = eventBase & {
    EventId: 1;
    Name: string;
    RaceId: number;
}

export type eventDamage = eventBase & {
    EventId: 3;
    TargetId: string;
    SkillId: number;
    Damage: number;
    IsCritical: boolean;
}

export type eventCharacterConditionEnable = eventBase & {
    EventId: 4;
    CCId: number;
    DisableAt: number;
    AttackerId: string;
}

export type eventCharacterConditionDisable = eventBase & {
    EventId: 5;
    CCId: number;
}

export type eventFinish = eventBase & {
    EventId: 6;
    AttackerId: string;
}
