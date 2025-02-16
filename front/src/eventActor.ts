import * as protocols from '@/protocols';

// TODO: entity와 dmanage store 분리하기, entity appear에서 damage store에 있던 정보들 가져오기
// TODO: take cc, apply cc 구분하기

export class ActorManager {
    public entityMap: Record<string, EntityActor> = {};
    public groupMap: Record<string, GroupActor> = {};

    public static pcRaceSet = new Set<number>([8001, 8002, 9001, 9002, 10001, 10002]);

    public onEvent(event: protocols.eventBase) {
        if (event.EventId === protocols.eventIdEntityAppear) {
            this.onEntityAppear(event as protocols.eventEntityAppear);
        }

        const entity = this.entityMap[event.Id];

        switch (event.EventId) {
            case protocols.eventIdEntityAppear:
                entity.onEntityAppear(event as protocols.eventEntityAppear);
                break;

            case protocols.eventIdDamage:
                {
                    const event_ = event as protocols.eventDamage;

                    if (entity) {
                        entity.onApplyDamage(event_);
                        entity.group.onApplyDamage(event_);
                    }

                    const targetEntity = this.entityMap[event_.TargetId];
                    if (targetEntity) {
                        targetEntity.onTakeDamage(event_);
                        targetEntity.group.onTakeDamage(event_);
                    }
                }
                break;

            case protocols.eventIdCharacterConditionEnable:
                if (!entity) {
                    return;
                }

                entity.onCharacterConditionEnable(event as protocols.eventCharacterConditionEnable);
                break;

            case protocols.eventIdCharacterConditionDisable:
                if (!entity) {
                    return;
                }

                entity.onCharacterConditionDisable(event as protocols.eventCharacterConditionDisable);
                break;

            case protocols.eventIdFinish:
                if (!entity) {
                    return;
                }

                entity.onFinish(event as protocols.eventFinish);
                break;
        }
    }

    public onEntityAppear(event: protocols.eventEntityAppear) {
        const { Id, RaceId, Name } = event;

        const groupKey = ActorManager.groupTargetKey(event);
        const group = this.groupMap[groupKey] ??= new GroupActor(this, groupKey, RaceId);

        this.entityMap[Id] = group.entityMap[Id] ??= new EntityActor(this, Id, RaceId, Name, group);

    }

    public clear() {
        // object instance를 새로 만들면 귀찮아짐

        for (const k in this.entityMap) {
            const v = this.entityMap[k];
            
            v.clear();
        }

        for (const k in this.groupMap) {
            const v = this.groupMap[k];

            v.clear();
        }
    }

    private static groupTargetKey(event: protocols.eventEntityAppear): string {
        // pc 일 경우 group안에 entity가 여러개 생기지 않도록
        if (this.pcRaceSet.has(event.RaceId)) {
            return event.Id;
        }

        return `${event.RaceId}`;
    }
}

interface IEventActor {
    /** damage 쪽 수치들만 reset */
    clear(): void;
}

export abstract class BaseActor implements IEventActor {
    protected constructor(protected mgr: ActorManager, private _id: string, private _raceId: number, private _name: string) {
        this._isPC = ActorManager.pcRaceSet.has(_raceId);
    }

    public get id() {
        return this._id;
    }

    public get raceId() {
        return this._raceId;
    }

    public get name() {
        return this._name;
    }

    /** 받은 대미지 */
    public get totalTakeDamage() {
        return this._totalTakeDamage;
    }
    protected _totalTakeDamage = 0;

    protected _takeDamages: EntityDamage[] = [];
    public get takeDamages() {
        return this._takeDamages;
    }

    /** 준 대미지 */
    public get totalApplyDamage() {
        return this._totalApplyDamage;
    }
    protected _totalApplyDamage = 0;

    protected _applyDamages: EntityDamage[] = [];
    public get applyDamages() {
        return this._applyDamages;
    }

    private _isPC = false;
    public get isPC() {
        return this._isPC;
    }

    public onEntityAppear(event: protocols.eventEntityAppear): void {
        // nothing
        event;
    }

    public onTakeDamage(event: protocols.eventDamage): void {
        // nothing
        event;
    }

    public onApplyDamage(event: protocols.eventDamage): void {
        // nothing
        event;
    }

    public onCharacterConditionEnable(event: protocols.eventCharacterConditionEnable): void {
        // nothing
        event;
    }

    public onCharacterConditionDisable(event: protocols.eventCharacterConditionDisable): void {
        // nothing
        event;
    }

    public onFinish(event: protocols.eventFinish): void {
        // nothing
        event;
    }

    public clear() {
        this._totalTakeDamage = 0;
        this._takeDamages.length = 0;

        this._totalApplyDamage = 0;
        this._applyDamages.length = 0;
    }
}

export class EntityActor extends BaseActor {
    public constructor(mgr: ActorManager, id: string, raceId: number, name: string, private _group: GroupActor) {
        super(mgr, id, raceId, name);
    }

    public get group() {
        return this._group;
    }

    protected _conditionMap: Record<number, EntityCondition> = {};
    public get conditionMap() {
        return this._conditionMap;
    }

    private _finisherId = '';
    public get finisherId() {
        return this._finisherId;
    }

    private _totalApplyDamageByTarget: Record<string, number> = {};
    public get totalApplyDamageByTarget() {
        return this._totalApplyDamageByTarget;
    }

    private _totalTakeDamageByAttacker: Record<string, number> = {};
    public get totalTakeDamageByAttacker() {
        return this._totalTakeDamageByAttacker;
    }

    public override onEntityAppear(event: protocols.eventEntityAppear): void {
        this._finisherId = '';

        if (ActorManager.pcRaceSet.has(event.RaceId)) {
            // pc일 경우 damage 초기와 안함
            return;
        }

        this._totalTakeDamage = 0;
        this._takeDamages.length = 0;
    }

    public override onTakeDamage(event: protocols.eventDamage): void {
        const attacker = this.mgr.entityMap[event.Id];

        const damage: EntityDamage = {
            ...event,

            // copy
            Conditions: attacker ? Object.values(attacker.conditionMap) : [],
            TargetConditions: Object.values(this._conditionMap),
        }

        this._totalTakeDamage += event.Damage;
        this._takeDamages.push(damage);

        this._totalTakeDamageByAttacker[event.Id] ??= 0;
        this._totalTakeDamageByAttacker[event.Id] += event.Damage;
    }

    public override onApplyDamage(event: protocols.eventDamage): void {
        const targetId = event.TargetId;
        const target = this.mgr.entityMap[targetId];
        if (!target || !(target instanceof EntityActor)) {
            // 몹 정보가 없으면 무시
            return;
        }

        const damage: EntityDamage = {
            ...event,

            // copy
            Conditions: Object.values(this._conditionMap),
            TargetConditions: Object.values(target.conditionMap),
        }

        this._totalApplyDamageByTarget[targetId] ??= 0;
        this._totalApplyDamageByTarget[targetId] += event.Damage;

        this._totalApplyDamage += event.Damage;
        this._applyDamages.push(damage);
    }

    public override onCharacterConditionEnable(event: protocols.eventCharacterConditionEnable): void {
        this._conditionMap[event.CCId] = {
            Id: event.Id,
            CCId: event.CCId,
            DisableAt: event.DisableAt,
            AttackerId: event.AttackerId,
        };
    }

    public override onCharacterConditionDisable(event: protocols.eventCharacterConditionDisable): void {
        delete this._conditionMap[event.CCId];
    }

    public override onFinish(event: protocols.eventFinish): void {
        this._finisherId = event.AttackerId;
    }

    public override clear() {
        super.clear();

        this._totalApplyDamageByTarget = {};
        this._totalTakeDamageByAttacker = {};
    }
}

// TODO: GroupActor에 Group 조건 추가하는 식으로 바꾸는게 좋을듯
export class GroupActor extends BaseActor {
    public constructor(mgr: ActorManager, id: string, raceId: number) {
        const groupName = `${raceId}`;
        super(mgr, id, raceId, groupName);
    }

    private _entityMap: Record<string, EntityActor> = {};
    public get entityMap() {
        return this._entityMap;
    }

    private _totalTakeDamageByAttacker: Record<string, number> = {};
    public get totalTakeDamageByAttacker() {
        return this._totalTakeDamageByAttacker;
    }

    public override onEntityAppear(event: protocols.eventEntityAppear): void {
        if (ActorManager.pcRaceSet.has(event.RaceId)) {
            // pc일 경우 damage 초기와 안함
            return;
        }

        const target = this._entityMap[event.Id];
        if (!target) {
            // ?
            return;
        }

        this._takeDamages = this._takeDamages.filter(v => v.Id !== event.Id);
        this._totalTakeDamage = this._takeDamages.reduce((acc, v) => acc + v.Damage, 0);

        this._totalTakeDamageByAttacker = {};
        for (const v of this._takeDamages) {
            this._totalTakeDamageByAttacker[v.Id] ??= 0;
            this._totalTakeDamageByAttacker[v.Id] += v.Damage;
        }
    }

    public override onTakeDamage(event: protocols.eventDamage): void {
        // console.log('group take damage', this.id, event);

        const attacker = this.mgr.entityMap[event.Id];
        const target = this._entityMap[event.TargetId];
        if (!target) {
            // ?
            return;
        }

        const damage: EntityDamage = {
            ...event,

            // copy
            Conditions: attacker ? Object.values(attacker.conditionMap) : [],
            TargetConditions: Object.values(target.conditionMap),
        }

        this._totalTakeDamage += event.Damage;
        this._takeDamages.push(damage);

        this._totalTakeDamageByAttacker[event.Id] ??= 0;
        this._totalTakeDamageByAttacker[event.Id] += event.Damage;
    }

    public override clear(): void {
        super.clear();

        this._totalTakeDamageByAttacker = {};
    }
}

export type EntityDamage = {
    Id: string;
    TargetId: string;
    SkillId: number;
    Damage: number;
    IsCritical: boolean;
    Conditions: EntityCondition[];
    TargetConditions: EntityCondition[];
};

export type EntityCondition = {
    Id: string;
    CCId: number;
    DisableAt: number;
    AttackerId: string;
}